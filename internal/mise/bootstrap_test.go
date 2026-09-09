// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// signedInstaller is a real install.sh.sig as published at mise.jdx.dev,
// signed by the release key the package embeds.
func signedInstaller(t *testing.T) []byte {
	t.Helper()
	sig, err := os.ReadFile("testdata/install.sh.sig")
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

// TestVerifyInstaller pins the trust root: the published signed installer
// verifies and yields the script; a flipped byte, a truncated message, and
// plain unsigned bytes are all ErrBadSignature.
func TestVerifyInstaller(t *testing.T) {
	sig := signedInstaller(t)
	script, err := VerifyInstaller(sig)
	if err != nil {
		t.Fatalf("VerifyInstaller(published): %v", err)
	}
	if !strings.HasPrefix(string(script), "#!/bin/sh") || !strings.Contains(string(script), "sha256") {
		t.Errorf("verified script does not look like the installer:\n%.200s", script)
	}

	tampered := slices.Clone(sig)
	tampered[len(tampered)/2] ^= 0x01
	cases := map[string][]byte{
		"tampered":  tampered,
		"truncated": sig[:len(sig)-40],
		"unsigned":  []byte("#!/bin/sh\necho pwned\n"),
		"empty":     nil,
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := VerifyInstaller(bad)
			if !errors.Is(err, ErrBadSignature) {
				t.Errorf("error = %v, want ErrBadSignature", err)
			}
			if got != nil {
				t.Errorf("returned %d bytes of an unverified script", len(got))
			}
		})
	}
}

// TestEnsureInstalled pins the binary preference: the installer's
// ~/.local/bin/mise, else a PATH hit outside the Homebrew prefixes, else the
// verified installer — which runs without a profile's MISE_CONFIG_DIR and
// targets ~/.local/bin.
func TestEnsureInstalled(t *testing.T) {
	cases := []struct {
		name        string
		local       bool
		pathHit     string
		wantBin     string // "" means the local path
		wantInstall bool
	}{
		{name: "local bin wins", local: true, pathHit: "/opt/homebrew/bin/mise"},
		{name: "PATH hit outside brew", pathHit: "/usr/local/bin/mise", wantBin: "/usr/local/bin/mise"},
		{name: "brew keg is not used", pathHit: "/opt/homebrew/bin/mise", wantInstall: true},
		{name: "nothing found installs", wantInstall: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			local := LocalBin(home)
			if c.local {
				if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(local, []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			lookPath := func(string) (string, error) {
				if c.pathHit == "" {
					return "", errors.New("not found")
				}
				return c.pathHit, nil
			}
			fetched := 0
			fetch := func(_ context.Context, url string) ([]byte, error) {
				fetched++
				if url != installerSigURL {
					t.Errorf("fetched %s, want %s", url, installerSigURL)
				}
				return signedInstaller(t), nil
			}
			r := &fakeRunner{}
			got, err := EnsureInstalled(context.Background(), r, lookPath, fetch, home)
			if err != nil {
				t.Fatalf("EnsureInstalled: %v", err)
			}
			want := c.wantBin
			if want == "" {
				want = local
			}
			if got != want {
				t.Errorf("bin = %q, want %q", got, want)
			}
			if !c.wantInstall {
				if len(r.calls) != 0 || fetched != 0 {
					t.Errorf("calls = %v, fetched %d; want nothing", r.calls, fetched)
				}
				return
			}
			if len(r.calls) != 1 {
				t.Fatalf("calls = %v, want one installer run", r.calls)
			}
			assertInstallerRun(t, r.calls[0], local)
		})
	}

	t.Run("bad signature never runs", func(t *testing.T) {
		home := t.TempDir()
		lookPath := func(string) (string, error) { return "", errors.New("not found") }
		fetch := func(context.Context, string) ([]byte, error) { return []byte("echo pwned"), nil }
		r := &fakeRunner{}
		_, err := EnsureInstalled(context.Background(), r, lookPath, fetch, home)
		if !errors.Is(err, ErrBadSignature) {
			t.Errorf("error = %v, want ErrBadSignature", err)
		}
		if len(r.calls) != 0 {
			t.Errorf("calls = %v, want none for an unverified installer", r.calls)
		}
	})
}

// assertInstallerRun checks one verified-installer invocation: sh on a
// scratch script that is removed afterwards, targeting local without a
// profile's config dir, with ~/.local/bin created for it.
func assertInstallerRun(t *testing.T, call call, local string) {
	t.Helper()
	if call.kind != "interactive" || len(call.argv) != 2 || call.argv[0] != "sh" ||
		!strings.HasSuffix(call.argv[1], ".sh") {
		t.Errorf("installer call = %+v, want sh <script>", call)
	}
	if !slices.Contains(call.env, "MISE_INSTALL_PATH="+local) {
		t.Errorf("installer env = %v, want MISE_INSTALL_PATH=%s", call.env, local)
	}
	for _, e := range call.env {
		if strings.HasPrefix(e, "MISE_CONFIG_DIR=") {
			t.Errorf("installer env carries a config dir: %v", call.env)
		}
	}
	if _, err := os.Stat(call.argv[1]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("scratch installer %s left behind: %v", call.argv[1], err)
	}
	if _, err := os.Stat(filepath.Dir(local)); err != nil {
		t.Errorf("~/.local/bin not created: %v", err)
	}
}

func TestLookupNotInstalled(t *testing.T) {
	_, err := Lookup(func(string) (string, error) { return "", errors.New("no") }, t.TempDir())
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("error = %v, want ErrNotInstalled", err)
	}
}
