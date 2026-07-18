// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestMergedLocalBranches(t *testing.T) {
	ctx := context.Background()
	trunk := Trunk{Remote: "upstream", Branch: "main"}

	t.Run("excludes trunk and keeps merged features", func(t *testing.T) {
		var gotArgs []string
		r := &fakeRunner{output: func(args []string) ([]byte, error) {
			gotArgs = args
			return []byte("feat-1\nmain\nfeat-2\n"), nil
		}}
		got, err := MergedLocalBranches(ctx, r, trunk)
		if err != nil {
			t.Fatalf("MergedLocalBranches() error: %v", err)
		}
		if want := []string{"feat-1", "feat-2"}; !slices.Equal(got, want) {
			t.Errorf("MergedLocalBranches() = %v, want %v", got, want)
		}
		want := []string{"branch", "--merged", "upstream/main", "--format=%(refname:short)"}
		if !slices.Equal(gotArgs, want) {
			t.Errorf("git args = %v, want %v", gotArgs, want)
		}
	})

	t.Run("propagates git failure", func(t *testing.T) {
		sentinel := errors.New("git exploded")
		r := &fakeRunner{output: func([]string) ([]byte, error) { return nil, sentinel }}
		if _, err := MergedLocalBranches(ctx, r, trunk); !errors.Is(err, sentinel) {
			t.Errorf("MergedLocalBranches() error = %v, want wrapped %v", err, sentinel)
		}
	})
}

func TestMergedRemoteBranches(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		trunk  Trunk
		output string
		want   []string
	}{
		{
			name:  "skips HEAD, trunk, and other remotes",
			trunk: Trunk{Remote: "upstream", Branch: "main"},
			// origin/HEAD shortens to bare "origin" on some git versions; both
			// spellings must be skipped.
			output: "origin\norigin/HEAD\norigin/main\norigin/feat-1\nupstream/main\norigin/feat-2\n",
			want:   []string{"feat-1", "feat-2"},
		},
		{
			name:   "trunk on origin",
			trunk:  Trunk{Remote: "origin", Branch: "main"},
			output: "origin/main\norigin/feat-1\n",
			want:   []string{"feat-1"},
		},
		{
			name:   "nothing merged",
			trunk:  Trunk{Remote: "origin", Branch: "main"},
			output: "origin/main\n",
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			r := &fakeRunner{output: func(args []string) ([]byte, error) {
				gotArgs = args
				return []byte(tt.output), nil
			}}
			got, err := MergedRemoteBranches(ctx, r, "origin", tt.trunk)
			if err != nil {
				t.Fatalf("MergedRemoteBranches() error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("MergedRemoteBranches() = %v, want %v", got, tt.want)
			}
			want := []string{"branch", "-r", "--merged", tt.trunk.Ref(), "--format=%(refname:short)"}
			if !slices.Equal(gotArgs, want) {
				t.Errorf("git args = %v, want %v", gotArgs, want)
			}
		})
	}
}

func TestListStacks(t *testing.T) {
	ctx := context.Background()

	t.Run("loads each recorded stack ordered by id", func(t *testing.T) {
		values := map[string]string{
			"dotty.stack.beta.layers":      "feat-b",
			"dotty.stack.alpha.layers":     "feat-1,feat-2",
			"dotty.branch.feat-1.pr":       "41",
			"dotty.branch.feat-1.titlesha": "",
		}
		r := &fakeRunner{output: func(args []string) ([]byte, error) {
			if slices.Contains(args, "--get-regexp") {
				return []byte("dotty.stack.beta.layers feat-b\ndotty.stack.alpha.layers feat-1,feat-2\n"), nil
			}
			// configGet: key is the final argument.
			return []byte(values[args[len(args)-1]] + "\n"), nil
		}}
		stacks, err := ListStacks(ctx, r)
		if err != nil {
			t.Fatalf("ListStacks() error: %v", err)
		}
		if len(stacks) != 2 || stacks[0].ID != "alpha" || stacks[1].ID != "beta" {
			t.Fatalf("ListStacks() ids = %v, want [alpha beta]", stacks)
		}
		branches := make([]string, 0, len(stacks[0].Layers))
		for _, l := range stacks[0].Layers {
			branches = append(branches, l.Branch)
		}
		if want := []string{"feat-1", "feat-2"}; !slices.Equal(branches, want) {
			t.Errorf("stack alpha layers = %v, want %v", branches, want)
		}
		if stacks[0].Layers[0].PR != 41 {
			t.Errorf("feat-1 PR = %d, want 41", stacks[0].Layers[0].PR)
		}
	})

	t.Run("no recorded stacks reads as empty, not an error", func(t *testing.T) {
		r := &fakeRunner{output: func([]string) ([]byte, error) {
			return nil, exitErr(1)
		}}
		stacks, err := ListStacks(ctx, r)
		if err != nil {
			t.Fatalf("ListStacks() error: %v", err)
		}
		if len(stacks) != 0 {
			t.Errorf("ListStacks() = %v, want empty", stacks)
		}
	})

	t.Run("propagates a genuine git failure", func(t *testing.T) {
		r := &fakeRunner{output: func([]string) ([]byte, error) {
			return nil, exitErr(128)
		}}
		if _, err := ListStacks(ctx, r); err == nil || !strings.Contains(err.Error(), "list stacks") {
			t.Errorf("ListStacks() error = %v, want wrapped list stacks failure", err)
		}
	})
}
