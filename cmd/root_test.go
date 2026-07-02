// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/securitykey"
)

// execDotty runs the real command tree, as a user invocation would.
func execDotty(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(context.Background())
}

// TestFlagBeforeVerb pins DESIGN's ergonomics: noun-level persistent flags
// parse before or after the verb, with '=' or space separators, by running
// `security-key add` end-to-end against a scratch data dir.
func TestFlagBeforeVerb(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "flag before verb, equals form", args: []string{"security-key", "--serial=111", "add", "--name=k1"}},
		{name: "flag before verb, space form", args: []string{"security-key", "--serial", "222", "add", "--name", "k2"}},
		{name: "flag after verb", args: []string{"security-key", "add", "--serial=333", "--name=k3"}},
		{name: "noun alias sk", args: []string{"sk", "--serial=444", "add", "--name=k4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataDir)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			if err := execDotty(t, tt.args...); err != nil {
				t.Fatalf("execute %v: %v", tt.args, err)
			}

			store, err := securitykey.LoadStore(securitykey.StorePath(filepath.Join(dataDir, "dotty")))
			if err != nil {
				t.Fatal(err)
			}
			if names := store.Names(); len(names) != 1 {
				t.Errorf("store names = %v, want exactly one alias", names)
			}
			// Reset the persistent flag value for the next case.
			securityKeyFlags.Serial = ""
			securityKeyAddFlags.Name = ""
		})
	}
}

// TestDispatchArgs pins the SSH-entry argv rewrites: gpg.ssh.program may be the
// dotty binary itself (git passes -Y first) or a dotty-ssh-sign symlink, and an
// $SSH_ASKPASS invocation is recognized by the DOTTY_ASKPASS sentinel.
func TestDispatchArgs(t *testing.T) {
	noEnv := func(string) string { return "" }
	askPassEnv := func(k string) string {
		if k == "DOTTY_ASKPASS" {
			return "1"
		}
		return ""
	}
	tests := []struct {
		name   string
		argv   []string
		getenv func(string) string
		want   []string
	}{
		{
			name:   "plain invocation untouched",
			argv:   []string{"/usr/local/bin/dotty", "profile", "new"},
			getenv: noEnv,
			want:   []string{"profile", "new"},
		},
		{
			name:   "git -Y invocation rewritten",
			argv:   []string{"/usr/local/bin/dotty", "-Y", "sign", "-n", "git", "-f", "/k", "/buf"},
			getenv: noEnv,
			want:   []string{"signing-key", "sign", "-Y", "sign", "-n", "git", "-f", "/k", "/buf"},
		},
		{
			name:   "shim symlink rewritten",
			argv:   []string{"/Users/x/.local/bin/dotty-ssh-sign", "-Y", "sign", "-f", "/k", "/buf"},
			getenv: noEnv,
			want:   []string{"signing-key", "sign", "-Y", "sign", "-f", "/k", "/buf"},
		},
		{
			name:   "askpass invocation rewritten by sentinel",
			argv:   []string{"/usr/local/bin/dotty", "Enter PIN for ED25519-SK key: "},
			getenv: askPassEnv,
			want:   []string{"signing-key", "ask-pass", "Enter PIN for ED25519-SK key: "},
		},
		{
			// A globally-exported SSH_ASKPASS points ssh-keygen at the
			// dotty-ssh-askpass symlink; the basename routes it here even without
			// the sentinel, which the `new`/`import` ssh-keygen child never carries.
			name:   "askpass shim basename rewritten without sentinel",
			argv:   []string{"/Users/x/.local/share/scripts/dotty-ssh-askpass", "Enter PIN for authenticator: "},
			getenv: noEnv,
			want:   []string{"signing-key", "ask-pass", "Enter PIN for authenticator: "},
		},
		{
			// The shim basename wins over the -Y sign heuristic: a PIN prompt that
			// happens to begin with -Y must still reach ask-pass, not sign.
			name:   "askpass shim wins over a -Y-looking prompt",
			argv:   []string{"/Users/x/.local/share/scripts/dotty-ssh-askpass", "-Y is a prompt here"},
			getenv: noEnv,
			want:   []string{"signing-key", "ask-pass", "-Y is a prompt here"},
		},
		{
			name: "askpass sentinel wins over a prompt that looks like a shim",
			argv: []string{"/Users/x/.local/bin/dotty-ssh-sign", "-Y is not a sign request here"},
			// A PIN prompt reaching the dotty-ssh-sign path must still go to
			// ask-pass, not be mistaken for a sign invocation.
			getenv: askPassEnv,
			want:   []string{"signing-key", "ask-pass", "-Y is not a sign request here"},
		},
		{
			name:   "bare invocation untouched",
			argv:   []string{"dotty"},
			getenv: noEnv,
			want:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dispatchArgs(tt.argv, tt.getenv); !slices.Equal(got, tt.want) {
				t.Errorf("dispatchArgs(%v) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}
