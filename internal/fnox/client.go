// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fnox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// Bin is the fnox executable name.
const Bin = "fnox"

// scratchKey is the throwaway secret SetKeychainItem routes a provider write
// through: fnox only writes to a provider as a side effect of setting a
// secret, so the item is created under this name and the TOML entry dropped
// straight after. The provider-side item survives the removal.
const scratchKey = "DOTTY_BOOTSTRAP_SCRATCH"

// Runner is the slice of *cli.ExecRunner the client needs: captured output
// with and without stdin or extra environment, and PATH lookup.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	OutputEnv(ctx context.Context, extraEnv []string, name string, args ...string) ([]byte, error)
	OutputStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

// Target names the config file and profile an operation addresses. Global
// selects the global config (fnox's -g on writes; the file is passed with -c
// on every call so the working directory's own fnox.toml never gets in the
// way). Config is an explicit project file, e.g. /repo/fnox.toml — fnox
// refuses a -c path that does not exist, so callers create it first. An
// empty Profile is the top-level [secrets] table; otherwise the
// [profiles.<name>.secrets] table.
type Target struct {
	Global  bool
	Config  string
	Profile string
}

// String describes the target for messages: "the global config",
// "/repo/fnox.toml", either suffixed with the profile when one is set.
func (t Target) String() string {
	var b strings.Builder
	if t.Global {
		b.WriteString("the global config")
	} else {
		b.WriteString(t.Config)
	}
	if t.Profile != "" {
		b.WriteString(" (profile ")
		b.WriteString(t.Profile)
		b.WriteString(")")
	}
	return b.String()
}

// Client drives the fnox CLI through a Runner.
type Client struct {
	r      Runner
	global string
}

// New returns a client whose global targets address globalConfig — normally
// GlobalConfigPath(). A nil r panics on first use; there is no sensible
// default for a process runner.
func New(r Runner, globalConfig string) *Client {
	return &Client{r: r, global: globalConfig}
}

// GlobalConfig returns the global config file path the client addresses.
func (c *Client) GlobalConfig() string { return c.global }

// Ensure reports whether fnox is on PATH, with the install hint dotty's own
// package set provides.
func (c *Client) Ensure() error {
	if _, err := c.r.LookPath(Bin); err != nil {
		return fmt.Errorf("%s is required; dotty packages sync installs it: %w", Bin, err)
	}
	return nil
}

// args assembles argv for verb against t: fnox's global flags (-c, -P) go
// before the verb, the write-scope -g right after it, then extra.
func (c *Client) args(t Target, write bool, verb string, extra ...string) []string {
	argv := make([]string, 0, 8+len(extra))
	if cfg := c.configPath(t); cfg != "" {
		argv = append(argv, "-c", cfg)
	}
	if t.Profile != "" {
		argv = append(argv, "-P", t.Profile)
	}
	argv = append(argv, verb)
	if write && t.Global {
		argv = append(argv, "-g")
	}
	return append(argv, extra...)
}

// configPath is the -c value for t: the global file for a global target,
// else the project file (empty leaves fnox to its own search).
func (c *Client) configPath(t Target) string {
	if t.Global {
		return c.global
	}
	return t.Config
}

// Set stores value under key in t, encrypted or held by provider (fnox's
// default_provider when empty). The value travels on stdin, never in argv.
// fnox trims surrounding whitespace from a stdin value; callers that care
// warn before calling.
func (c *Client) Set(ctx context.Context, t Target, provider, key string, value []byte) error {
	return c.set(ctx, t, provider, key, value)
}

// set is Set with extra flags spliced in before the key.
func (c *Client) set(ctx context.Context, t Target, provider, key string, value []byte, flags ...string) error {
	extra := make([]string, 0, len(flags)+3)
	if provider != "" {
		extra = append(extra, "-p", provider)
	}
	extra = append(extra, flags...)
	extra = append(extra, key)
	if _, err := c.r.OutputStdin(ctx, value, Bin, c.args(t, true, "set", extra...)...); err != nil {
		return fmt.Errorf("set %s in %s: %w", key, t, err)
	}
	return nil
}

// SetDefault records key in t with a plaintext default and no provider
// value — fnox's form for a non-secret literal. A metadata-only set never
// reads stdin, so the value goes in argv; callers pass only values that were
// already plaintext on disk.
func (c *Client) SetDefault(ctx context.Context, t Target, key, def string) error {
	argv := c.args(t, true, "set", "--default="+def, key)
	if _, err := c.r.Output(ctx, Bin, argv...); err != nil {
		return fmt.Errorf("set default for %s in %s: %w", key, t, err)
	}
	return nil
}

// Remove drops key's entry from t's config. fnox edits the TOML only; a
// provider-side item (keychain, remote store) is left in place.
func (c *Client) Remove(ctx context.Context, t Target, key string) error {
	if _, err := c.r.Output(ctx, Bin, c.args(t, true, "remove", key)...); err != nil {
		return fmt.Errorf("remove %s from %s: %w", key, t, err)
	}
	return nil
}

// Keys lists the secret names defined in t itself: --no-defaults keeps the
// top-level table out of a profile listing, and a project target runs with
// FNOX_CONFIG_DIR pointed at an empty directory so global secrets are not
// merged in. A profile that does not exist yet has no keys rather than
// being an error.
func (c *Client) Keys(ctx context.Context, t Target) ([]string, error) {
	if t.Profile != "" {
		profiles, err := c.Profiles(ctx, t)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(profiles, t.Profile) {
			return nil, nil
		}
	}
	argv := c.args(t, false, "list", "--complete")
	argv = slices.Insert(argv, len(argv)-2, "--no-defaults")
	out, err := c.run(ctx, t, argv)
	if err != nil {
		return nil, fmt.Errorf("list keys in %s: %w", t, err)
	}
	return lines(out), nil
}

// Profiles lists the profile names t's config defines (the implicit default
// included).
func (c *Client) Profiles(ctx context.Context, t Target) ([]string, error) {
	argv := c.args(Target{Global: t.Global, Config: t.Config}, false, "profiles", "--complete")
	out, err := c.run(ctx, t, argv)
	if err != nil {
		return nil, fmt.Errorf("list profiles in %s: %w", t, err)
	}
	return lines(out), nil
}

// Providers lists the provider names the global config defines; none when
// the file does not exist yet.
func (c *Client) Providers(ctx context.Context) ([]string, error) {
	if _, err := os.Stat(c.global); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	out, err := c.r.Output(ctx, Bin, "-c", c.global, "provider", "list")
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	return lines(out), nil
}

// SetKeychainItem writes value into provider (a keychain-type provider)
// under keyName without leaving a secret behind in the config: the write
// goes through a scratch secret that is removed immediately after. The
// keychain item itself survives, which is what an age identity reference
// needs.
func (c *Client) SetKeychainItem(ctx context.Context, provider, keyName string, value []byte) error {
	t := Target{Global: true}
	if err := c.set(ctx, t, provider, scratchKey, value, "-k", keyName); err != nil {
		return err
	}
	return c.Remove(ctx, t, scratchKey)
}

// run executes a read verb. Project targets are isolated from the global
// config so their listings reflect the one file; global targets already
// address that file explicitly.
func (c *Client) run(ctx context.Context, t Target, argv []string) ([]byte, error) {
	if t.Global {
		return c.r.Output(ctx, Bin, argv...)
	}
	dir, err := os.MkdirTemp("", "dotty-fnox-*")
	if err != nil {
		return nil, fmt.Errorf("create scratch config dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return c.r.OutputEnv(ctx, []string{"FNOX_CONFIG_DIR=" + dir}, Bin, argv...)
}

// lines splits captured output into its non-blank, trimmed lines.
func lines(out []byte) []string {
	var result []string
	for l := range strings.Lines(string(out)) {
		if l = strings.TrimSpace(l); l != "" {
			result = append(result, l)
		}
	}
	return result
}
