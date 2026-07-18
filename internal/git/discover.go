// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// minStackNodes is the shortest chain (including trunk) that counts as a stack.
// Two nodes is trunk + one branch — just a branch, not a stack.
const minStackNodes = 3

// ListLocalBranches returns short names of local branches.
func ListLocalBranches(ctx context.Context, r Runner) ([]string, error) {
	out, err := r.Output(ctx, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("list local branches: %w", err)
	}
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// TrunkTip resolves a revision for the trunk tip: prefer the remote-tracking
// ref, fall back to a local branch of the same name.
func TrunkTip(ctx context.Context, r Runner, trunk Trunk) (string, error) {
	if _, err := RevParse(ctx, r, trunk.Ref()); err == nil {
		return trunk.Ref(), nil
	}
	if _, err := RevParse(ctx, r, trunk.Branch); err == nil {
		return trunk.Branch, nil
	}
	return "", fmt.Errorf("cannot resolve trunk tip (%s or %s)", trunk.Ref(), trunk.Branch)
}

// tip is a feature branch candidate: a local branch name and its tip SHA.
type tip struct {
	name string
	sha  string
}

// DiscoverStack finds an obvious linear stack containing current by inspecting
// local branch tips. A stack is a chain trunk → … → tip of length ≥ 3 nodes
// (trunk + at least two feature branches). Returns ok=false when nothing
// obvious is found (including a lone branch off trunk).
//
// Does not write config — caller may SaveStack when ok.
func DiscoverStack(ctx context.Context, r Runner, trunk Trunk, current string) (Stack, bool, error) {
	trunkTip, err := TrunkTip(ctx, r, trunk)
	if err != nil {
		return Stack{}, false, err
	}
	trunkSHA, err := RevParse(ctx, r, trunkTip)
	if err != nil {
		return Stack{}, false, err
	}

	all, err := ListLocalBranches(ctx, r)
	if err != nil {
		return Stack{}, false, err
	}
	features := featureTips(ctx, r, trunk, trunkSHA, all)
	if len(features) < 2 {
		// Need at least two feature branches for a 3-node chain with trunk.
		return Stack{}, false, nil
	}

	closestParent := closestParents(ctx, r, features, trunkSHA)
	if _, ok := closestParent[current]; !ok {
		// Current is not a feature candidate (maybe trunk itself).
		return Stack{}, false, nil
	}
	path, err := lineagePath(closestParent, current, len(features)+1)
	if err != nil {
		return Stack{}, false, err
	}
	// Nodes = trunk + path; require ≥ 3.
	if len(path)+1 < minStackNodes {
		return Stack{}, false, nil
	}

	id, err := newStackID()
	if err != nil {
		return Stack{}, false, err
	}
	s := Stack{ID: id, Layers: make([]Layer, 0, len(path))}
	for _, name := range path {
		hint, _ := CommitSubject(ctx, r, name)
		s.Layers = append(s.Layers, Layer{Branch: name, TitleHint: hint})
	}
	return s, true, nil
}

// featureTips returns local branches whose tips are strict descendants of the
// trunk tip — the candidates that can form a stack.
func featureTips(ctx context.Context, r Runner, trunk Trunk, trunkSHA string, all []string) []tip {
	var features []tip
	for _, name := range all {
		if name == trunk.Branch || name == "main" || name == "master" {
			continue
		}
		sha, err := RevParse(ctx, r, name)
		if err != nil {
			continue
		}
		if sha == trunkSHA {
			continue // empty branch at trunk tip
		}
		onTrunk, err := IsAncestor(ctx, r, trunkSHA, sha)
		if err != nil || !onTrunk {
			continue // not FF from trunk (diverged or unrelated)
		}
		features = append(features, tip{name: name, sha: sha})
	}
	return features
}

// closestParents maps each feature branch to its nearest ancestor feature;
// the empty string means the parent is trunk.
func closestParents(ctx context.Context, r Runner, features []tip, trunkSHA string) map[string]string {
	closestParent := map[string]string{}
	for _, b := range features {
		parent := ""
		parentSHA := trunkSHA
		for _, a := range features {
			if a.name == b.name || a.sha == b.sha {
				continue
			}
			// a is under b (strict ancestor of b's tip)
			isAnc, err := IsAncestor(ctx, r, a.sha, b.sha)
			if err != nil || !isAnc {
				continue
			}
			// Prefer the a that is closest to b: parentSHA must be ancestor of a.sha
			// (a is further from trunk / closer to b than current parent).
			closer, err := IsAncestor(ctx, r, parentSHA, a.sha)
			if err == nil && closer {
				parent = a.name
				parentSHA = a.sha
			}
		}
		closestParent[b.name] = parent
	}
	return closestParent
}

// lineagePath walks closestParent from current down to trunk, reverses the
// chain to bottom → … → current, then extends it toward the tip while the
// chain stays unambiguous (exactly one child). maxLen bounds the walk so a
// corrupt parent map cannot loop forever.
func lineagePath(closestParent map[string]string, current string, maxLen int) ([]string, error) {
	var path []string
	for b := current; b != ""; b = closestParent[b] {
		path = append(path, b)
		if len(path) > maxLen {
			return nil, errors.New("cycle detected while discovering stack lineage")
		}
	}
	slices.Reverse(path)

	children := map[string][]string{} // parent key → children names
	for child, par := range closestParent {
		children[par] = append(children[par], child)
	}
	for {
		kids := children[path[len(path)-1]]
		if len(kids) != 1 || slices.Contains(path, kids[0]) {
			break
		}
		path = append(path, kids[0])
	}
	return path, nil
}

// LoadOrDiscoverStack loads configured stack for the current branch, or when
// none is configured tries DiscoverStack and persists it when found.
func LoadOrDiscoverStack(ctx context.Context, r Runner, trunk Trunk) (Stack, bool, error) {
	cur, err := CurrentBranch(ctx, r)
	if err != nil {
		return Stack{}, false, err
	}
	s, err := LoadStack(ctx, r, cur)
	if err == nil {
		return s, false, nil
	}
	if !errors.Is(err, ErrNotInStack) {
		return Stack{}, false, err
	}
	s, ok, err := DiscoverStack(ctx, r, trunk, cur)
	if err != nil {
		return Stack{}, false, err
	}
	if !ok {
		return Stack{}, false, fmt.Errorf("branch %q is not in a stack (use `dotty git start`, "+
			"or create a local lineage of trunk→…→tip with at least two feature branches)", cur)
	}
	if err := SaveStack(ctx, r, s); err != nil {
		return Stack{}, false, fmt.Errorf("save discovered stack: %w", err)
	}
	return s, true, nil
}
