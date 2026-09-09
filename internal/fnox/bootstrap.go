// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fnox

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// The provider pair Bootstrap writes. AgeProvider encrypts every value;
// KeychainProvider holds its software identity under IdentityKey so
// decryption never prompts.
const (
	AgeProvider      = "dotty-age"
	KeychainProvider = "dotty-keychain"
	IdentityKey      = "dotty-age-key"
	keychainService  = "fnox"
	ageKeygenBin     = "age-keygen"
)

// configTmpl renders the global config: the exec-only env mode, the default
// provider, and the two provider tables as named sub-templates so a present
// file can receive just the tables it lacks.
//
//go:embed config.toml.tmpl
var configTmplSrc string

var configTmpl = template.Must(template.New("config").Parse(configTmplSrc))

// configData feeds configTmpl.
type configData struct {
	AgeProvider      string
	KeychainProvider string
	KeychainService  string
	IdentityKey      string
	Recipients       []string
}

// envSettingRe and defaultProvRe find the two top-level settings a
// hand-written global config must carry for dotty's model to hold; Bootstrap
// checks the region before the first table for them.
var (
	envSettingRe  = regexp.MustCompile(`(?m)^\s*env\s*=`)
	defaultProvRe = regexp.MustCompile(`(?m)^\s*default_provider\s*=`)
)

// age-keygen prints the recipient as a "# public key:" comment on stdout and
// a "Public key:" notice on stderr, and the identity as the AGE-SECRET-KEY-1
// line.
const (
	secretKeyPrefix = "AGE-SECRET-KEY-1"
)

var recipientMarkers = []string{"# public key:", "Public key:"}

// Bootstrap makes the global config ready for dotty's use and is safe to
// re-run: with AgeProvider already defined nothing is touched. Otherwise it
// generates a software age identity, writes the provider pair (rendering the
// whole file when absent, appending only the missing tables when present),
// and parks the identity in the keychain through KeychainProvider. recovery
// lists extra recipients — the YubiKeys enrolled for the private dotfiles —
// so any of them can decrypt the values on a machine without this identity.
func Bootstrap(ctx context.Context, ios cli.IOStreams, c *Client, recovery []string) error {
	if err := c.Ensure(); err != nil {
		return err
	}
	if _, err := c.r.LookPath(ageKeygenBin); err != nil {
		return fmt.Errorf("%s is required to create the software identity: %w", ageKeygenBin, err)
	}
	providers, err := c.Providers(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(providers, AgeProvider) {
		tui.Infof(ios, "fnox provider %s already configured in %s", AgeProvider, c.global)
		return nil
	}

	out, err := c.r.Output(ctx, ageKeygenBin)
	if err != nil {
		return err
	}
	pub, identity, err := parseKeygen(out)
	if err != nil {
		return err
	}
	data := configData{
		AgeProvider:      AgeProvider,
		KeychainProvider: KeychainProvider,
		KeychainService:  keychainService,
		IdentityKey:      IdentityKey,
		Recipients:       append([]string{pub}, recovery...),
	}
	if len(recovery) == 0 {
		tui.Infof(ios, "No recovery recipients: re-run dotty env migrate after enrolling a security key to add them")
	}

	if err := writeConfig(ios, c.global, providers, data); err != nil {
		return err
	}
	// The identity write goes through the provider the file now defines.
	if err := c.SetKeychainItem(ctx, KeychainProvider, IdentityKey, identity); err != nil {
		return err
	}
	tui.Successf(ios, "fnox configured in %s: %s encrypts to %d recipient%s, identity parked in the keychain as %q",
		c.global, AgeProvider, len(data.Recipients), plural(len(data.Recipients)), IdentityKey)
	return nil
}

// writeConfig renders a fresh global config, or appends the provider tables
// an existing one lacks. Appending valid TOML tables to the end of a file is
// always valid TOML, so the present file is never parsed; the two top-level
// settings it should carry are checked textually and reported for the user
// to add by hand.
func writeConfig(ios cli.IOStreams, path string, providers []string, data configData) error {
	existing, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		var b bytes.Buffer
		if err := configTmpl.Execute(&b, data); err != nil {
			return fmt.Errorf("render fnox config: %w", err)
		}
		if err := cli.EnsureDir(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return cli.AtomicWriteFile(path, b.Bytes(), 0o600)
	case err != nil:
		return fmt.Errorf("read %s: %w", path, err)
	}

	var b bytes.Buffer
	b.Write(existing)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		b.WriteString("\n")
	}
	for _, name := range []string{KeychainProvider, AgeProvider} {
		if slices.Contains(providers, name) {
			continue
		}
		b.WriteString("\n")
		tmpl := "keychain"
		if name == AgeProvider {
			tmpl = "age"
		}
		if err := configTmpl.ExecuteTemplate(&b, tmpl, data); err != nil {
			return fmt.Errorf("render fnox provider %s: %w", name, err)
		}
	}
	if err := cli.AtomicWriteFile(path, b.Bytes(), 0o600); err != nil {
		return err
	}

	head, _, _ := strings.Cut(string(existing), "\n[")
	if !envSettingRe.MatchString(head) || !defaultProvRe.MatchString(head) {
		tui.Warnf(ios, "%s predates dotty; add these two lines above its first table:\n"+
			"  env = \"exec\"\n  default_provider = %q", path, AgeProvider)
	}
	return nil
}

// parseKeygen extracts the recipient and the secret key line from age-keygen
// output ("# public key: age1…" then "AGE-SECRET-KEY-1…"). Only the key line
// is kept as the identity: age parses it as a one-identity file, and the
// comments carry nothing the keychain item needs.
func parseKeygen(out []byte) (pub string, identity []byte, err error) {
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		for _, marker := range recipientMarkers {
			if rest, ok := strings.CutPrefix(line, marker); ok {
				if fields := strings.Fields(rest); len(fields) > 0 {
					pub = fields[0]
				}
			}
		}
		if strings.HasPrefix(line, secretKeyPrefix) {
			identity = []byte(strings.Fields(line)[0])
		}
	}
	switch {
	case !strings.HasPrefix(pub, "age1"):
		return "", nil, fmt.Errorf("no public key in %s output", ageKeygenBin)
	case identity == nil:
		return "", nil, fmt.Errorf("no secret key in %s output", ageKeygenBin)
	}
	return pub, identity, nil
}

// plural is the "s" suffix for counts other than one.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
