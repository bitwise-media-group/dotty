// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// valuelessTrue marks a key stored without a value (`[section] key`), which
// git defines as boolean true.
const valuelessTrue = "\x00valueless"

// fakeGitConfig fakes the two git config invocations ConfigLookup and
// ConfigLookupBool issue, canonicalizing booleans the way git does.
type fakeGitConfig struct {
	values map[string]string
	keys   []string // every key queried, raw and typed lookups alike
}

func (f *fakeGitConfig) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := args[len(args)-1]
	f.keys = append(f.keys, key)
	raw, ok := f.values[key]
	if !slices.Contains(args, "--type=bool") {
		if !ok || raw == valuelessTrue {
			return []byte("\n"), nil
		}
		return []byte(raw + "\n"), nil
	}
	switch {
	case !ok:
		return []byte("false\n"), nil // --default "" canonicalized
	case raw == valuelessTrue:
		return []byte("true\n"), nil
	}
	switch strings.ToLower(raw) {
	case "true", "yes", "on", "1":
		return []byte("true\n"), nil
	case "false", "no", "off", "0":
		return []byte("false\n"), nil
	}
	return nil, fmt.Errorf("bad boolean config value %q", raw)
}

func (f *fakeGitConfig) Run(context.Context, string, ...string) error            { return nil }
func (f *fakeGitConfig) RunInteractive(context.Context, string, ...string) error { return nil }

// newConfigTestCmd builds a throwaway verb shaped like the real ones: a mix
// of flag types, one excluded flag, and one hidden flag.
func newConfigTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "propose"}
	cmd.Flags().Bool("browse", false, "")
	cmd.Flags().Bool("copy", false, "")
	var am autoMergeMode
	cmd.Flags().Var(&am, "auto-merge", "")
	cmd.Flags().String("remote", "origin", "")
	cmd.Flags().Int("up", 1, "")
	cmd.Flags().Bool("root", false, "")
	excludeGitConfigFlags(cmd.Flags(), "root")
	cmd.Flags().Bool("internal", false, "")
	if err := cmd.Flags().MarkHidden("internal"); err != nil {
		t.Fatal(err)
	}
	cmd.InitDefaultHelpFlag()
	return cmd
}

func TestApplyGitConfigFlagDefaults(t *testing.T) {
	cases := []struct {
		name       string
		config     map[string]string
		args       []string // command line, parsed before defaults apply
		wantErrSub string
		check      func(t *testing.T, cmd *cobra.Command, fake *fakeGitConfig)
	}{
		{
			name:   "bool flag reads a config default without marking it changed",
			config: map[string]string{"dotty.propose.browse": "yes"},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got, _ := cmd.Flags().GetBool("browse"); !got {
					t.Errorf("browse = false, want true from config")
				}
				if cmd.Flags().Changed("browse") {
					t.Errorf("browse marked changed; config must read as a default, not an argument")
				}
			},
		},
		{
			name:   "valueless key reads true",
			config: map[string]string{"dotty.propose.copy": valuelessTrue},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got, _ := cmd.Flags().GetBool("copy"); !got {
					t.Errorf("copy = false, want true from a valueless key")
				}
			},
		},
		{
			name:   "command line wins over config",
			config: map[string]string{"dotty.propose.browse": "true"},
			args:   []string{"--browse=false"},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got, _ := cmd.Flags().GetBool("browse"); got {
					t.Errorf("browse = true, want the explicit --browse=false to win")
				}
			},
		},
		{
			name:   "non-bool flags read config values",
			config: map[string]string{"dotty.propose.remote": "upstream", "dotty.propose.up": "3"},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got, _ := cmd.Flags().GetString("remote"); got != "upstream" {
					t.Errorf("remote = %q, want upstream", got)
				}
				if got, _ := cmd.Flags().GetInt("up"); got != 3 {
					t.Errorf("up = %d, want 3", got)
				}
			},
		},
		{
			name:   "excluded flag is never queried",
			config: map[string]string{"dotty.propose.root": "true"},
			check: func(t *testing.T, cmd *cobra.Command, fake *fakeGitConfig) {
				if got, _ := cmd.Flags().GetBool("root"); got {
					t.Errorf("root = true; an excluded flag must ignore config")
				}
				if slices.Contains(fake.keys, "dotty.propose.root") {
					t.Errorf("dotty.propose.root was queried; excluded flags must not touch git config")
				}
			},
		},
		{
			name:   "hidden and help flags are never queried",
			config: map[string]string{"dotty.propose.internal": "true", "dotty.propose.help": "true"},
			check: func(t *testing.T, cmd *cobra.Command, fake *fakeGitConfig) {
				for _, key := range []string{"dotty.propose.internal", "dotty.propose.help"} {
					if slices.Contains(fake.keys, key) {
						t.Errorf("%s was queried; hidden/help flags must not touch git config", key)
					}
				}
			},
		},
		{
			name:   "custom-value flag reads config through its own vocabulary",
			config: map[string]string{"dotty.propose.auto-merge": "comment"},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got := cmd.Flags().Lookup("auto-merge").Value.String(); got != string(autoMergeComment) {
					t.Errorf("auto-merge = %q, want %q", got, autoMergeComment)
				}
			},
		},
		{
			name:   "custom-value flag reads a merge method from config",
			config: map[string]string{"dotty.propose.auto-merge": "rebase"},
			check: func(t *testing.T, cmd *cobra.Command, _ *fakeGitConfig) {
				if got := cmd.Flags().Lookup("auto-merge").Value.String(); got != "rebase" {
					t.Errorf("auto-merge = %q, want %q", got, "rebase")
				}
			},
		},
		{
			name:       "invalid auto-merge mode errors with the key",
			config:     map[string]string{"dotty.propose.auto-merge": "banana"},
			wantErrSub: "dotty.propose.auto-merge",
		},
		{
			name:       "unparseable value errors with the key",
			config:     map[string]string{"dotty.propose.up": "lots"},
			wantErrSub: "dotty.propose.up",
		},
		{
			name:       "unreadable boolean errors with the key",
			config:     map[string]string{"dotty.propose.browse": "banana"},
			wantErrSub: "dotty.propose.browse",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := newConfigTestCmd(t)
			if err := cmd.ParseFlags(c.args); err != nil {
				t.Fatal(err)
			}
			fake := &fakeGitConfig{values: c.config}
			err := applyGitConfigFlagDefaults(context.Background(), fake, cmd)
			if c.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
					t.Fatalf("applyGitConfigFlagDefaults() error = %v, want substring %q", err, c.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyGitConfigFlagDefaults() error: %v", err)
			}
			c.check(t, cmd, fake)
		})
	}
}

// TestGitConfigDefaultsWiring pins the mechanism to the real command tree:
// every git verb runs it via the parent's PersistentPreRunE, and the flags
// where a persistent default is destructive (resign --root, worktree remove
// --yes/--force) or meaningless
// (merge --up, a stack-relative quantity) are opted out.
func TestGitConfigDefaultsWiring(t *testing.T) {
	if gitCmd.PersistentPreRunE == nil {
		t.Error("gitCmd.PersistentPreRunE is nil; git verbs would never read config defaults")
	}
	excluded := []struct {
		verb *cobra.Command
		flag string
	}{
		{gitResignCmd, "root"},
		{gitMergeCmd, "up"},
		{gitWorktreeRemoveCmd, "yes"},
		{gitWorktreeRemoveCmd, "force"},
	}
	for _, e := range excluded {
		f := e.verb.Flags().Lookup(e.flag)
		if f == nil {
			t.Fatalf("%s has no --%s flag", e.verb.Name(), e.flag)
		}
		if _, ok := f.Annotations[noGitConfigAnnotation]; !ok {
			t.Errorf("%s --%s is not excluded from git-config defaults", e.verb.Name(), e.flag)
		}
	}
}
