// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Layer is one branch in a stack, ordered bottom (nearest trunk) to tip.
type Layer struct {
	Branch    string
	PR        int    // 0 if none
	TitleSHA  string // commit chosen for PR title/body; empty if unset
	TitleHint string // cached subject for status/viz; may be stale
}

// Stack is a linear chain of layers with a stable id.
type Stack struct {
	ID     string
	Layers []Layer // index 0 = bottom
}

// IndexOf returns the index of branch in the stack, or -1.
func (s Stack) IndexOf(branch string) int {
	return slices.IndexFunc(s.Layers, func(l Layer) bool { return l.Branch == branch })
}

// Tip is the topmost layer branch name.
func (s Stack) Tip() string {
	if len(s.Layers) == 0 {
		return ""
	}
	return s.Layers[len(s.Layers)-1].Branch
}

// Bottom is the first layer branch name.
func (s Stack) Bottom() string {
	if len(s.Layers) == 0 {
		return ""
	}
	return s.Layers[0].Branch
}

// ErrNotInStack reports a branch with no stack lineage recorded. Callers
// branch on it with errors.Is — to fall back to discovery, or to tell the
// user to run `dotty git start`.
var ErrNotInStack = errors.New("not in a stack")

// LoadStack reads stack id for branch and returns the full stack, or an error
// wrapping ErrNotInStack if the branch is not part of a stack.
func LoadStack(ctx context.Context, r Runner, branch string) (Stack, error) {
	id, err := configGet(ctx, r, branchStackKey(branch))
	if err != nil {
		return Stack{}, fmt.Errorf("read stack lineage for %s: %w", branch, err)
	}
	if id == "" {
		return Stack{}, fmt.Errorf("branch %q is %w (use `dotty git start`)", branch, ErrNotInStack)
	}
	return loadStackByID(ctx, r, id)
}

// LoadStackForHEAD loads the stack containing the current branch.
func LoadStackForHEAD(ctx context.Context, r Runner) (Stack, error) {
	b, err := CurrentBranch(ctx, r)
	if err != nil {
		return Stack{}, err
	}
	return LoadStack(ctx, r, b)
}

func loadStackByID(ctx context.Context, r Runner, id string) (Stack, error) {
	raw, err := configGet(ctx, r, stackLayersKey(id))
	if err != nil {
		return Stack{}, fmt.Errorf("read layers for stack %s: %w", id, err)
	}
	if raw == "" {
		return Stack{}, fmt.Errorf("stack %s has no layers: %w", id, ErrNotInStack)
	}
	names := splitCSV(raw)
	s := Stack{ID: id, Layers: make([]Layer, 0, len(names))}
	for _, name := range names {
		l := Layer{Branch: name}
		if pr, err := configGet(ctx, r, branchPRKey(name)); err == nil && pr != "" {
			if n, err := strconv.Atoi(pr); err == nil {
				l.PR = n
			}
		}
		if sha, err := configGet(ctx, r, branchTitleSHAKey(name)); err == nil {
			l.TitleSHA = sha
		}
		if hint, err := configGet(ctx, r, branchTitleHintKey(name)); err == nil {
			l.TitleHint = hint
		}
		s.Layers = append(s.Layers, l)
	}
	return s, nil
}

// Start creates a new branch from trunk and registers it as a one-layer stack.
func Start(ctx context.Context, r Runner, trunk Trunk, branch string) (Stack, error) {
	if branch == "" {
		return Stack{}, errors.New("branch name is required")
	}
	if err := ensureBranchName(branch); err != nil {
		return Stack{}, err
	}
	// A failed fetch is tolerated (offline) as long as the trunk ref still
	// resolves locally.
	if err := FetchTrunk(ctx, r, trunk); err != nil {
		if _, e := RevParse(ctx, r, trunk.Ref()); e != nil {
			return Stack{}, fmt.Errorf("fetch %s: %w", trunk.Ref(), err)
		}
	}
	if err := r.Run(ctx, "git", "checkout", "-b", branch, trunk.Ref()); err != nil {
		return Stack{}, fmt.Errorf("create branch %s from %s: %w", branch, trunk.Ref(), err)
	}
	// Track origin/<branch> from birth so a bare `git push` / `git pull`
	// targets the push remote before the first push creates it. A repo with
	// no origin (nothing to push to yet) just skips this.
	if remote, rerr := PushRemote(ctx, r); rerr == nil {
		if err := SetUpstream(ctx, r, remote, branch); err != nil {
			return Stack{}, err
		}
	}
	id, err := newStackID()
	if err != nil {
		return Stack{}, err
	}
	s := Stack{ID: id, Layers: []Layer{{Branch: branch}}}
	if err := saveStack(ctx, r, s); err != nil {
		return Stack{}, err
	}
	return s, nil
}

// AdoptBranch registers an existing branch that has no recorded lineage as a
// new single-layer stack, so single-branch workflows can use stack commands
// (propose, sync) without having started the branch via `dotty git start`.
func AdoptBranch(ctx context.Context, r Runner, branch string) (Stack, error) {
	if branch == "" {
		return Stack{}, errors.New("branch name is required")
	}
	if err := ensureBranchName(branch); err != nil {
		return Stack{}, err
	}
	id, err := newStackID()
	if err != nil {
		return Stack{}, err
	}
	s := Stack{ID: id, Layers: []Layer{{Branch: branch}}}
	if err := saveStack(ctx, r, s); err != nil {
		return Stack{}, err
	}
	return s, nil
}

// Append creates a child branch from HEAD and adds it as the new tip of the
// stack that contains the current branch.
func Append(ctx context.Context, r Runner, child string) (Stack, error) {
	if child == "" {
		return Stack{}, errors.New("branch name is required")
	}
	if err := ensureBranchName(child); err != nil {
		return Stack{}, err
	}
	cur, err := CurrentBranch(ctx, r)
	if err != nil {
		return Stack{}, err
	}
	s, err := LoadStack(ctx, r, cur)
	if err != nil {
		return Stack{}, err
	}
	if s.IndexOf(child) >= 0 {
		return Stack{}, fmt.Errorf("branch %q is already in this stack", child)
	}
	// Current must be the tip to append (keep lineage linear and simple).
	if cur != s.Tip() {
		return Stack{}, fmt.Errorf("check out the stack tip %q before appending (or use `dotty git up`)", s.Tip())
	}
	if err := r.Run(ctx, "git", "checkout", "-b", child); err != nil {
		return Stack{}, fmt.Errorf("create branch %s: %w", child, err)
	}
	// Track origin/<child> from birth so a bare `git push` / `git pull`
	// targets the push remote before the first push creates it. A repo with
	// no origin (nothing to push to yet) just skips this.
	if remote, rerr := PushRemote(ctx, r); rerr == nil {
		if err := SetUpstream(ctx, r, remote, child); err != nil {
			return Stack{}, err
		}
	}
	s.Layers = append(s.Layers, Layer{Branch: child})
	if err := saveStack(ctx, r, s); err != nil {
		return Stack{}, err
	}
	return s, nil
}

// SaveStack writes the full stack metadata to local git config.
func SaveStack(ctx context.Context, r Runner, s Stack) error {
	return saveStack(ctx, r, s)
}

func saveStack(ctx context.Context, r Runner, s Stack) error {
	if s.ID == "" {
		return errors.New("stack id is empty")
	}
	names := make([]string, len(s.Layers))
	for i, l := range s.Layers {
		names[i] = l.Branch
		if err := configSet(ctx, r, branchStackKey(l.Branch), s.ID); err != nil {
			return err
		}
		if l.PR > 0 {
			if err := configSet(ctx, r, branchPRKey(l.Branch), strconv.Itoa(l.PR)); err != nil {
				return err
			}
		}
		if l.TitleSHA != "" {
			if err := configSet(ctx, r, branchTitleSHAKey(l.Branch), l.TitleSHA); err != nil {
				return err
			}
		}
		if l.TitleHint != "" {
			if err := configSet(ctx, r, branchTitleHintKey(l.Branch), l.TitleHint); err != nil {
				return err
			}
		}
	}
	return configSet(ctx, r, stackLayersKey(s.ID), strings.Join(names, ","))
}

// RemoveLayer drops a branch from the stack metadata (after merge cleanup).
func RemoveLayer(ctx context.Context, r Runner, s Stack, branch string) (Stack, error) {
	idx := s.IndexOf(branch)
	if idx < 0 {
		return s, nil
	}
	s.Layers = slices.Delete(s.Layers, idx, idx+1)
	configUnset(ctx, r, branchStackKey(branch))
	configUnset(ctx, r, branchPRKey(branch))
	configUnset(ctx, r, branchTitleSHAKey(branch))
	configUnset(ctx, r, branchTitleHintKey(branch))
	if len(s.Layers) == 0 {
		configUnset(ctx, r, stackLayersKey(s.ID))
		return Stack{}, nil
	}
	if err := saveStack(ctx, r, s); err != nil {
		return Stack{}, err
	}
	return s, nil
}

// Checkout moves HEAD to branch.
func Checkout(ctx context.Context, r Runner, branch string) error {
	if err := r.Run(ctx, "git", "checkout", branch); err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// Up moves num layers toward the tip from current branch.
func Up(ctx context.Context, r Runner, num int) (string, error) {
	return move(ctx, r, max(num, 1))
}

// Down moves num layers toward the trunk from current branch.
func Down(ctx context.Context, r Runner, num int) (string, error) {
	return move(ctx, r, -max(num, 1))
}

// move checks out the layer delta positions from the current branch,
// clamped to the stack bounds.
func move(ctx context.Context, r Runner, delta int) (string, error) {
	cur, err := CurrentBranch(ctx, r)
	if err != nil {
		return "", err
	}
	s, err := LoadStack(ctx, r, cur)
	if err != nil {
		return "", err
	}
	i := s.IndexOf(cur)
	if i < 0 {
		return "", fmt.Errorf("current branch %q is %w", cur, ErrNotInStack)
	}
	i = min(max(i+delta, 0), len(s.Layers)-1)
	dest := s.Layers[i].Branch
	return dest, Checkout(ctx, r, dest)
}

func branchStackKey(branch string) string     { return "dotty.branch." + branch + ".stack" }
func branchPRKey(branch string) string        { return "dotty.branch." + branch + ".pr" }
func branchTitleSHAKey(branch string) string  { return "dotty.branch." + branch + ".titlesha" }
func branchTitleHintKey(branch string) string { return "dotty.branch." + branch + ".titlehint" }
func stackLayersKey(id string) string         { return "dotty.stack." + id + ".layers" }

// configGet reads a --local config value. An unset key is not an error: it
// yields "" (--default "" keeps git's exit status zero for the unset case, so
// a non-nil error is always a genuine git failure).
func configGet(ctx context.Context, r Runner, key string) (string, error) {
	out, err := r.Output(ctx, "git", "config", "--local", "--default", "", "--get", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func configSet(ctx context.Context, r Runner, key, value string) error {
	return r.Run(ctx, "git", "config", "--local", key, value)
}

// configUnset removes a key, ignoring failure when it is already missing.
func configUnset(ctx context.Context, r Runner, key string) {
	_ = r.Run(ctx, "git", "config", "--local", "--unset", key)
}

func newStackID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func splitCSV(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ensureBranchName(name string) error {
	if strings.ContainsAny(name, " \t\n~^:?*[\\") {
		return fmt.Errorf("invalid branch name %q", name)
	}
	if name == "main" || name == "master" || name == "HEAD" {
		return fmt.Errorf("refusing to use %q as a stack branch", name)
	}
	return nil
}
