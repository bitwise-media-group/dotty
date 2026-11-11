// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package wizard

import (
	"testing"

	"github.com/bitwise-media-group/dotty/internal/scaffold"
)

// TestMergeAnswersWorktrees pins the re-run worktree migration: a stored
// legacy in-repo default moves to the shared default, while an explicit
// flag — even one naming the legacy location — and a custom stored answer
// stay as given.
func TestMergeAnswersWorktrees(t *testing.T) {
	cases := []struct {
		name   string
		stored string
		flag   string
		want   string
	}{
		{"legacy default migrates", scaffold.LegacyDefaultWorktrees, "", scaffold.DefaultWorktrees},
		{"explicit legacy flag kept", scaffold.LegacyDefaultWorktrees, ".worktrees", ".worktrees"},
		{"flag overrides stored", "~/wt", "/abs/wt", "/abs/wt"},
		{"custom relative kept", "wt", "", "wt"},
		{"custom absolute kept", "~/wt", "", "~/wt"},
		{"current default kept", scaffold.DefaultWorktrees, "", scaffold.DefaultWorktrees},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prev := scaffold.Answers{Worktrees: c.stored}
			got := mergeAnswers(prev, Flags{Worktrees: c.flag}, "/home/x/Repos", "/home/x/Repos/dots", "/home/x")
			if got.Worktrees != c.want {
				t.Errorf("mergeAnswers(stored %q, flag %q).Worktrees = %q, want %q",
					c.stored, c.flag, got.Worktrees, c.want)
			}
		})
	}
}
