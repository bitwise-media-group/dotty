// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package worktree

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"dotty-fix", "dotty-fix"},
		{"my repo!", "my-repo-"},
		{".dotfiles", "dot-dotfiles"},
		{"trailing.", "trailing-dot"},
		{"a.b.c", "a-dot-b-dot-c"},
		{"Under_score-9", "Under_score-9"},
	}
	for _, tt := range tests {
		if got := Sanitize(tt.in); got != tt.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRoot(t *testing.T) {
	tests := []struct{ setting, want string }{
		{"", "/home/x/.local/share/dotty/worktrees"},
		{".worktrees", "/r/repo/.worktrees"},
		{"wt", "/r/repo/wt"},
		{"~/.cache/agent/worktrees", "/home/x/.cache/agent/worktrees"},
		{"/var/worktrees", "/var/worktrees"},
	}
	for _, tt := range tests {
		if got := Root("/r/repo", tt.setting, "/home/x"); got != tt.want {
			t.Errorf("Root(%q) = %q, want %q", tt.setting, got, tt.want)
		}
	}
}

func TestDerive(t *testing.T) {
	n := Derive("/r/my.repo", "fix bug", "/r/my.repo/.worktrees")
	if n.Name != "my-dot-repo-fix-bug" {
		t.Errorf("Name = %q", n.Name)
	}
	if n.Branch != "agent/my-dot-repo-fix-bug" {
		t.Errorf("Branch = %q", n.Branch)
	}
	if n.Path != "/r/my.repo/.worktrees/my-dot-repo-fix-bug" {
		t.Errorf("Path = %q", n.Path)
	}
}

func TestIsAgentBranch(t *testing.T) {
	if !IsAgentBranch("agent/x") || IsAgentBranch("main") || IsAgentBranch("feature/agent") {
		t.Error("IsAgentBranch misclassifies")
	}
}

func TestHookJSON(t *testing.T) {
	if got := ParseStartName([]byte(`{"name":"fix-1"}`)); got != "fix-1" {
		t.Errorf("ParseStartName = %q", got)
	}
	if got := ParseStartName([]byte(`broken`)); got != "" {
		t.Errorf("ParseStartName(broken) = %q", got)
	}
	if got := ParseEndPath([]byte(`{"worktree_path":"/w/x"}`)); got != "/w/x" {
		t.Errorf("ParseEndPath = %q", got)
	}
}
