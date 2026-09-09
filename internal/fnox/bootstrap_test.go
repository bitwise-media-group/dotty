// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fnox

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// writeFile is a test helper writing a small file 0600.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

const keygenOut = `# created: 2026-09-09T00:00:00Z
# public key: age1softwarekey
AGE-SECRET-KEY-1SOFTWARE
`

// bootstrapRunner scripts the fnox and age-keygen calls Bootstrap makes:
// provider list answers from providers, age-keygen from keygenOut, and
// every set/remove succeeds.
type bootstrapRunner struct {
	fakeRunner
	providers string
}

func (r *bootstrapRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{name: name, args: args})
	switch {
	case name == ageKeygenBin:
		return []byte(keygenOut), nil
	case len(args) >= 2 && args[len(args)-2] == "provider":
		return []byte(r.providers), nil
	}
	return nil, nil
}

// bootstrapEnv points the global config at a scratch dir and returns the
// path the file will land at plus a captured stderr.
func bootstrapEnv(t *testing.T) (string, cli.IOStreams, *bytes.Buffer) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "fnox")
	var errOut bytes.Buffer
	ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut}
	return filepath.Join(dir, ConfigFile), ios, &errOut
}

func TestBootstrapRendersAbsentConfig(t *testing.T) {
	cfg, ios, errOut := bootstrapEnv(t)
	r := &bootstrapRunner{}
	c := New(r, cfg)
	if err := Bootstrap(context.Background(), ios, c, []string{"age1yubikey1aaa", "age1yubikey1bbb"}); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	info, err := os.Stat(cfg)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 0600", perm)
	}
	if dirInfo, err := os.Stat(filepath.Dir(cfg)); err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("config dir mode = %v (err %v), want 0700", dirInfo, err)
	}
	got, _ := os.ReadFile(cfg)
	want := `# Written by dotty. Secrets are only injected into ` + "`fnox exec`" + ` subprocesses.
env = "exec"
default_provider = "dotty-age"

[providers.dotty-keychain]
type = "keychain"
service = "fnox"

[providers.dotty-age]
type = "age"
recipients = ["age1softwarekey", "age1yubikey1aaa", "age1yubikey1bbb"]
identity = { provider = "dotty-keychain", value = "dotty-age-key" }
`
	if string(got) != want {
		t.Errorf("config =\n%s\nwant\n%s", got, want)
	}

	// Keygen ran, then the identity was parked through the keychain
	// provider and the scratch entry removed.
	names := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		names = append(names, c.name+" "+strings.Join(c.args, " "))
	}
	joined := strings.Join(names, "\n")
	for _, sub := range []string{
		"age-keygen",
		"set -g -p dotty-keychain -k dotty-age-key " + scratchKey,
		"remove -g " + scratchKey,
	} {
		if !strings.Contains(joined, sub) {
			t.Errorf("calls lack %q:\n%s", sub, joined)
		}
	}
	for _, c := range r.calls {
		if c.stdin != "" && c.stdin != "AGE-SECRET-KEY-1SOFTWARE" {
			t.Errorf("unexpected stdin %q", c.stdin)
		}
	}
	if !strings.Contains(errOut.String(), "3 recipients") {
		t.Errorf("stderr = %q, want the recipient count", errOut.String())
	}
}

func TestBootstrapNoRecoveryRecipients(t *testing.T) {
	cfg, ios, errOut := bootstrapEnv(t)
	if err := Bootstrap(context.Background(), ios, New(&bootstrapRunner{}, cfg), nil); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got, _ := os.ReadFile(cfg)
	if !strings.Contains(string(got), `recipients = ["age1softwarekey"]`) {
		t.Errorf("config = %s, want the software key alone", got)
	}
	if !strings.Contains(errOut.String(), "No recovery recipients") {
		t.Errorf("stderr = %q, want the recovery hint", errOut.String())
	}
}

func TestBootstrapAppendsToPresentConfig(t *testing.T) {
	cfg, ios, errOut := bootstrapEnv(t)
	existing := "[providers.mine]\ntype = \"plain\"\n"
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(cfg, existing); err != nil {
		t.Fatal(err)
	}
	r := &bootstrapRunner{providers: "mine\n"}
	if err := Bootstrap(context.Background(), ios, New(r, cfg), nil); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got, _ := os.ReadFile(cfg)
	want := existing + `
[providers.dotty-keychain]
type = "keychain"
service = "fnox"

[providers.dotty-age]
type = "age"
recipients = ["age1softwarekey"]
identity = { provider = "dotty-keychain", value = "dotty-age-key" }
`
	if string(got) != want {
		t.Errorf("config =\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(errOut.String(), `env = "exec"`) || !strings.Contains(errOut.String(), "default_provider") {
		t.Errorf("stderr = %q, want the two settings to add by hand", errOut.String())
	}
}

func TestBootstrapAppendsOnlyMissingTable(t *testing.T) {
	cfg, ios, errOut := bootstrapEnv(t)
	existing := "env = \"exec\"\ndefault_provider = \"dotty-age\"\n\n" +
		"[providers.dotty-keychain]\ntype = \"keychain\"\nservice = \"fnox\""
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(cfg, existing); err != nil {
		t.Fatal(err)
	}
	r := &bootstrapRunner{providers: "dotty-keychain\n"}
	if err := Bootstrap(context.Background(), ios, New(r, cfg), nil); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got, _ := os.ReadFile(cfg)
	if strings.Count(string(got), "[providers.dotty-keychain]") != 1 {
		t.Errorf("keychain table duplicated:\n%s", got)
	}
	if !strings.Contains(string(got), "\n\n[providers.dotty-age]\n") {
		t.Errorf("age table not appended after a newline:\n%s", got)
	}
	if strings.Contains(errOut.String(), "predates dotty") {
		t.Errorf("stderr = %q, settings present so no warning expected", errOut.String())
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	cfg, ios, _ := bootstrapEnv(t)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(cfg, "env = \"exec\"\n"); err != nil {
		t.Fatal(err)
	}
	r := &bootstrapRunner{providers: "dotty-age\ndotty-keychain\n"}
	if err := Bootstrap(context.Background(), ios, New(r, cfg), nil); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, c := range r.calls {
		if c.name == ageKeygenBin {
			t.Error("age-keygen ran although the provider exists")
		}
		if len(c.args) > 0 && c.args[len(c.args)-1] == scratchKey {
			t.Error("identity rewritten although the provider exists")
		}
	}
	got, _ := os.ReadFile(cfg)
	if string(got) != "env = \"exec\"\n" {
		t.Errorf("config modified: %s", got)
	}
}

func TestBootstrapRequiresTools(t *testing.T) {
	cfg, ios, _ := bootstrapEnv(t)
	for _, missing := range []string{Bin, ageKeygenBin} {
		r := &bootstrapRunner{}
		r.missing = []string{missing}
		err := Bootstrap(context.Background(), ios, New(r, cfg), nil)
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("Bootstrap without %s = %v, want error naming it", missing, err)
		}
	}
}

func TestParseKeygen(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		wantPub string
		wantID  string
		wantErr bool
	}{
		{name: "stdout form", out: keygenOut, wantPub: "age1softwarekey", wantID: "AGE-SECRET-KEY-1SOFTWARE"},
		{name: "notice form", out: "Public key: age1abc\nAGE-SECRET-KEY-1X\n",
			wantPub: "age1abc", wantID: "AGE-SECRET-KEY-1X"},
		{name: "crlf", out: "# public key: age1abc\r\nAGE-SECRET-KEY-1X\r\n",
			wantPub: "age1abc", wantID: "AGE-SECRET-KEY-1X"},
		{name: "no public key", out: "AGE-SECRET-KEY-1X\n", wantErr: true},
		{name: "no secret key", out: "# public key: age1abc\n", wantErr: true},
		{name: "empty", out: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, id, err := parseKeygen([]byte(tt.out))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseKeygen(%q) = %q, %q; want error", tt.out, pub, id)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKeygen(%q): %v", tt.out, err)
			}
			if pub != tt.wantPub || string(id) != tt.wantID {
				t.Errorf("parseKeygen(%q) = %q, %q; want %q, %q", tt.out, pub, id, tt.wantPub, tt.wantID)
			}
		})
	}
}

func FuzzParseKeygen(f *testing.F) {
	for _, seed := range []string{
		keygenOut, "", "# public key:\n", "AGE-SECRET-KEY-1", "Public key: age1\nAGE-SECRET-KEY-1\n",
		"# public key:age1 0\nAGE-SECRET-KEY-1",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, out string) {
		pub, id, err := parseKeygen([]byte(out)) // must never panic
		if err != nil {
			return
		}
		// A success always yields a recipient and a one-line secret key.
		if !strings.HasPrefix(pub, "age1") || strings.ContainsAny(pub, " \t\r\n") {
			t.Errorf("parseKeygen(%q) accepted recipient %q", out, pub)
		}
		if !strings.HasPrefix(string(id), secretKeyPrefix) || strings.ContainsAny(string(id), "\r\n") {
			t.Errorf("parseKeygen(%q) accepted identity %q", out, id)
		}
	})
}
