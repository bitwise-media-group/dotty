// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// exitErr mimics *exec.ExitError just enough for exitCode: a plain error
// carrying a process exit code.
type exitErr int

func (e exitErr) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// memRunner is an in-memory fake for config + simple git queries.
type memRunner struct {
	config    map[string]string
	branch    string
	shas      map[string]string
	ancestors map[string]bool
	notIn     map[string]int // "rev|contained" → rev-list --count rev ^contained
	deleted   []string       // branches removed via git branch -d/-D
	remotes   string         // `git remote` output
	pushes    [][]string     // git push invocations
	pushErr   error          // returned by every git push
}

func newMem() *memRunner {
	return &memRunner{
		config:    map[string]string{},
		shas:      map[string]string{},
		ancestors: map[string]bool{},
		notIn:     map[string]int{},
		remotes:   "origin\nupstream\n",
	}
}

func (m *memRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	if name != "git" || len(args) == 0 {
		return nil, fmt.Errorf("unexpected %s %v", name, args)
	}
	switch args[0] {
	case "config":
		return m.configOutput(args)
	case "branch":
		return []byte(m.branch + "\n"), nil
	case "rev-parse":
		return m.revParseOutput(args)
	case "merge-base":
		return m.mergeBaseOutput(args)
	case "remote":
		return m.remoteOutput(args)
	case "rev-list":
		return m.revListOutput(args)
	}
	return nil, fmt.Errorf("unhandled git %v", args)
}

// revListOutput answers `git rev-list --count rev ^contained` from the notIn
// map; unlisted pairs count zero (contained).
func (m *memRunner) revListOutput(args []string) ([]byte, error) {
	if len(args) != 4 || args[1] != "--count" || !strings.HasPrefix(args[3], "^") {
		return nil, fmt.Errorf("unhandled git %v", args)
	}
	return fmt.Appendf(nil, "%d\n", m.notIn[args[2]+"|"+strings.TrimPrefix(args[3], "^")]), nil
}

// configOutput answers `git config [--local] --default "" --get key`: unset
// keys yield an empty line with a zero exit, matching real git.
func (m *memRunner) configOutput(args []string) ([]byte, error) {
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "--local" {
		rest = rest[1:]
	}
	if len(rest) != 4 || rest[0] != "--default" || rest[1] != "" || rest[2] != "--get" {
		return nil, fmt.Errorf("unhandled git %v", args)
	}
	return []byte(m.config[rest[3]] + "\n"), nil
}

func (m *memRunner) revParseOutput(args []string) ([]byte, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("unhandled git %v", args)
	}
	sha, ok := m.shas[args[1]]
	if !ok {
		return nil, fmt.Errorf("unknown rev %s", args[1])
	}
	return []byte(sha + "\n"), nil
}

func (m *memRunner) mergeBaseOutput(args []string) ([]byte, error) {
	if len(args) < 4 || args[1] != "--is-ancestor" {
		return nil, fmt.Errorf("unhandled git %v", args)
	}
	if m.ancestors[args[2]+"|"+args[3]] || args[2] == args[3] {
		return nil, nil
	}
	return nil, exitErr(1)
}

func (m *memRunner) remoteOutput(args []string) ([]byte, error) {
	if len(args) >= 3 && args[1] == "get-url" {
		return []byte("git@github.com:org/repo.git\n"), nil
	}
	return []byte(m.remotes), nil
}

func (m *memRunner) Run(_ context.Context, name string, args ...string) error {
	if name != "git" {
		return fmt.Errorf("unexpected %s", name)
	}
	// git config --local KEY VALUE  |  git config --local --unset KEY
	if len(args) >= 3 && args[0] == "config" && args[1] == "--local" {
		if args[2] == "--unset" && len(args) >= 4 {
			delete(m.config, args[3])
			return nil
		}
		if len(args) >= 4 {
			m.config[args[2]] = args[3]
			return nil
		}
	}
	if len(args) >= 2 && args[0] == "checkout" {
		// `checkout -b <name> [start-point]` lands on <name>, not the start.
		if args[1] == "-b" && len(args) >= 3 {
			m.branch = args[2]
		} else {
			m.branch = args[len(args)-1]
		}
		return nil
	}
	if args[0] == "push" {
		m.pushes = append(m.pushes, args)
		return m.pushErr
	}
	if len(args) >= 3 && args[0] == "branch" && (args[1] == "-d" || args[1] == "-D") {
		m.deleted = append(m.deleted, args[2])
		return nil
	}
	return nil
}

func (m *memRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return m.Run(ctx, name, args...)
}

func TestSaveLoadStack(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	s := Stack{
		ID: "abcd1234",
		Layers: []Layer{
			{Branch: "feat-a", PR: 1, TitleHint: "feat: a"},
			{Branch: "feat-b", PR: 2, TitleHint: "feat: b"},
		},
	}
	if err := SaveStack(ctx, m, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadStack(ctx, m, "feat-b")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || len(got.Layers) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got.Layers[0].PR != 1 || got.Layers[1].Branch != "feat-b" {
		t.Fatalf("layers %+v", got.Layers)
	}
	if got.IndexOf("feat-a") != 0 || got.Tip() != "feat-b" {
		t.Fatalf("index/tip")
	}
}

func TestAdoptBranch(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	s, err := AdoptBranch(ctx, m, "feat-solo")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" || len(s.Layers) != 1 || s.Layers[0].Branch != "feat-solo" {
		t.Fatalf("stack %+v", s)
	}
	got, err := LoadStack(ctx, m, "feat-solo")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.Tip() != "feat-solo" {
		t.Fatalf("got %+v", got)
	}
	for _, name := range []string{"", "main", "master", "HEAD"} {
		if _, err := AdoptBranch(ctx, m, name); err == nil {
			t.Fatalf("expected refusal for %q", name)
		}
	}
}

func TestClassifyTip(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	m.shas["upstream/main"] = "aaa"
	m.shas["feat"] = "bbb"
	m.ancestors["aaa|bbb"] = true
	rel, err := ClassifyTip(ctx, m, "upstream/main", "feat")
	if err != nil || rel != RelFF {
		t.Fatalf("ff: %v %v", rel, err)
	}
	m.shas["old"] = "ccc"
	m.ancestors["ccc|aaa"] = true
	rel, err = ClassifyTip(ctx, m, "upstream/main", "old")
	if err != nil || rel != RelMerged {
		t.Fatalf("merged: %v %v", rel, err)
	}
	m.shas["same"] = "aaa"
	rel, err = ClassifyTip(ctx, m, "upstream/main", "same")
	if err != nil || rel != RelIdentical {
		t.Fatalf("identical: %v %v", rel, err)
	}
	m.shas["div"] = "ddd"
	rel, err = ClassifyTip(ctx, m, "upstream/main", "div")
	if err != nil || rel != RelDiverged {
		t.Fatalf("diverged: %v %v", rel, err)
	}
}

func TestConfigKeysRoundTrip(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	if err := configSet(ctx, m, "dotty.stack.x.layers", "a,b"); err != nil {
		t.Fatal(err)
	}
	v, err := configGet(ctx, m, "dotty.stack.x.layers")
	if err != nil || v != "a,b" {
		t.Fatalf("got %q %v", v, err)
	}
	if !strings.Contains(branchStackKey("feat"), "feat") {
		t.Fatal("key")
	}
}

func TestStartSetsUpstream(t *testing.T) {
	m := newMem()
	trunk := Trunk{Remote: "origin", Branch: "main"}

	s, err := Start(context.Background(), m, trunk, "feat-1")
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if s.Tip() != "feat-1" {
		t.Errorf("stack tip = %q, want feat-1", s.Tip())
	}
	// The new branch tracks origin/feat-1 from birth, before any push exists.
	if got := m.config["branch.feat-1.remote"]; got != "origin" {
		t.Errorf("branch.feat-1.remote = %q, want origin", got)
	}
	if got := m.config["branch.feat-1.merge"]; got != "refs/heads/feat-1" {
		t.Errorf("branch.feat-1.merge = %q, want refs/heads/feat-1", got)
	}
}

func TestAppendSetsUpstream(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	trunk := Trunk{Remote: "origin", Branch: "main"}
	if _, err := Start(ctx, m, trunk, "feat-1"); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	s, err := Append(ctx, m, "feat-2")
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}
	if s.Tip() != "feat-2" {
		t.Errorf("stack tip = %q, want feat-2", s.Tip())
	}
	// The new layer tracks origin/feat-2 from birth, before any push exists.
	if got := m.config["branch.feat-2.remote"]; got != "origin" {
		t.Errorf("branch.feat-2.remote = %q, want origin", got)
	}
	if got := m.config["branch.feat-2.merge"]; got != "refs/heads/feat-2" {
		t.Errorf("branch.feat-2.merge = %q, want refs/heads/feat-2", got)
	}
}
