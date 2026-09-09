// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// FileOp is one file the render will produce: an embedded source written to
// a relative destination — under the shared repository, or under the
// profile's directory when PerProfile — or (when LinkTo is set) a
// repo-internal relative symlink.
type FileOp struct {
	Src        string // embed path; empty for symlink ops
	Dst        string // destination relative to the repo or profile dir
	LinkTo     string // symlink target relative to Dst's directory
	Mode       fs.FileMode
	Templated  bool
	PerProfile bool // renders into the profile's dir, not the repo root
	// KeepExisting renders the file only when the destination is missing:
	// the first render seeds it, after which the user owns it.
	KeepExisting bool
}

// MiseFragmentsDir is the profile-relative directory the selected
// components' mise fragments render into; mise merges every *.toml in it
// with the profile's config.toml.
const MiseFragmentsDir = "mise/conf.d"

// Plan resolves the answers' selections against the manifest and returns the
// file operations that build the repository, deterministically ordered.
func Plan(a Answers) ([]FileOp, error) {
	components, err := selected(a)
	if err != nil {
		return nil, err
	}

	var ops []FileOp
	for _, c := range components {
		for _, prefix := range c.Prefixes {
			err := fs.WalkDir(templateFS, prefix, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				ops = append(ops, newOp(path, strings.TrimPrefix(path, "template/")))
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("component %s: %w", c.ID, err)
			}
		}
		for src, dst := range c.Renames {
			if _, err := fs.Stat(templateFS, src); err != nil {
				return nil, fmt.Errorf("component %s: %w", c.ID, err)
			}
			ops = append(ops, newOp(src, dst))
		}
		if c.Mise != "" {
			if _, err := fs.Stat(templateFS, c.Mise); err != nil {
				return nil, fmt.Errorf("component %s: %w", c.ID, err)
			}
			ops = append(ops, FileOp{Src: c.Mise, Dst: path.Join(MiseFragmentsDir, path.Base(c.Mise)),
				Mode: 0o644, PerProfile: true})
		}
	}
	ops = append(ops, sharedDocOps(components)...)

	slices.SortFunc(ops, func(x, y FileOp) int { return strings.Compare(x.Dst, y.Dst) })
	// Components legitimately overlap (nvim pulls lazygit's config too);
	// identical claims collapse, contradictory ones are manifest bugs.
	deduped := ops[:0]
	for i, op := range ops {
		if i > 0 && op.Dst == ops[i-1].Dst {
			if op != ops[i-1] {
				return nil, fmt.Errorf("manifest maps %s from both %s and %s", op.Dst, ops[i-1].Src, op.Src)
			}
			continue
		}
		deduped = append(deduped, op)
	}
	return deduped, nil
}

// newOp builds the op for one embedded file, applying the templated,
// executable, and per-profile allowlists.
func newOp(src, dst string) FileOp {
	mode := fs.FileMode(0o644)
	if executable[src] {
		mode = 0o755
	}
	return FileOp{Src: src, Dst: dst, Mode: mode, Templated: templated[src], PerProfile: perProfile[src],
		KeepExisting: keepExisting[src]}
}

// sharedDocOps places the shared agent-memory doc: rendered once at the
// primary agent's path (claude's when selected — its tool demands the
// CLAUDE.md name — else the first selected agent's), with the remaining
// agents linking to it relatively so the copies cannot drift.
func sharedDocOps(components []Component) []FileOp {
	var agents []Component
	for _, c := range components {
		if c.Doc != "" {
			agents = append(agents, c)
		}
	}
	if len(agents) == 0 {
		return nil
	}
	primary := max(slices.IndexFunc(agents, func(c Component) bool { return c.ID == "agent:claude-code" }), 0)

	ops := []FileOp{newOp(sharedDoc, agents[primary].Doc)}
	for i, c := range agents {
		if i == primary {
			continue
		}
		rel, err := filepath.Rel(filepath.Dir(c.Doc), agents[primary].Doc)
		if err != nil {
			continue // both paths are repo-relative; Rel cannot fail in practice
		}
		ops = append(ops, FileOp{Dst: c.Doc, LinkTo: rel})
	}
	return ops
}

// Render writes ops into repoDir — or profileDir, the profile's directory,
// for per-profile files — substituting vars into templated files.
// Byte-copied files land exactly as embedded; templated files are rendered
// with missing-variable references treated as errors rather than silently
// emitted.
func Render(ops []FileOp, repoDir, profileDir string, v Vars) error {
	for _, op := range ops {
		root := repoDir
		if op.PerProfile {
			root = profileDir
		}
		dst := filepath.Join(root, op.Dst)
		if err := cli.EnsureDir(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if op.KeepExisting {
			if _, err := os.Lstat(dst); err == nil {
				continue
			}
		}
		if op.LinkTo != "" {
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("clear %s: %w", dst, err)
			}
			if err := os.Symlink(op.LinkTo, dst); err != nil {
				return fmt.Errorf("link %s: %w", dst, err)
			}
			continue
		}

		data, err := fs.ReadFile(templateFS, op.Src)
		if err != nil {
			return fmt.Errorf("read template %s: %w", op.Src, err)
		}
		if op.Templated {
			if data, err = render(op.Src, data, v); err != nil {
				return err
			}
		}
		if err := cli.AtomicWriteFile(dst, data, op.Mode); err != nil {
			return err
		}
	}
	return nil
}

// prunedRoots are the profile-relative trees that are wholly
// machine-rendered: an unplanned entry under them is never a user file.
var prunedRoots = []string{"home", MiseFragmentsDir}

// PrunePerProfile removes entries under the profile's rendered trees — the
// home/ tree and the mise fragments — that ops no longer render: orphans
// left behind when a template file relocates (~/.claude/settings.json →
// ~/.config/claude/settings.json) or a component is deselected. The trees
// are wholly machine-rendered, so an unplanned entry is never a user file;
// directories the pruning empties go too. It returns the pruned paths
// relative to the profile directory, so the caller can heal the live
// symlinks they leave dangling.
func PrunePerProfile(profileDir string, ops []FileOp) ([]string, error) {
	planned := make(map[string]bool, len(ops))
	for _, op := range ops {
		if op.PerProfile {
			planned[op.Dst] = true
		}
	}

	var pruned []string
	for _, rootRel := range prunedRoots {
		root := filepath.Join(profileDir, rootRel)
		var orphans []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(profileDir, path)
			if err != nil {
				return err
			}
			if !planned[rel] {
				orphans = append(orphans, path)
			}
			return nil
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue // no such tree yet — nothing to prune
		}
		if err != nil {
			return pruned, fmt.Errorf("prune profile renders: %w", err)
		}
		for _, orphan := range orphans {
			if err := os.Remove(orphan); err != nil {
				return pruned, fmt.Errorf("prune %s: %w", orphan, err)
			}
			rel, err := filepath.Rel(profileDir, orphan)
			if err != nil {
				return pruned, err
			}
			pruned = append(pruned, rel)
			for dir := filepath.Dir(orphan); dir != root; dir = filepath.Dir(dir) {
				if os.Remove(dir) != nil {
					break // still holds planned renders; so do its parents
				}
			}
		}
	}
	return pruned, nil
}

// render substitutes v into one templated file.
func render(name string, data []byte, v Vars) ([]byte, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// Unfold returns the $HOME-relative directories that must exist as real
// directories before linking, so tools writing runtime state beside their
// config never write through a folded symlink into the repository.
func Unfold(a Answers) ([]string, error) {
	components, err := selected(a)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, c := range components {
		dirs = append(dirs, c.Unfold...)
	}
	slices.Sort(dirs)
	return slices.Compact(dirs), nil
}

// selected resolves the answers to manifest components, rejecting unknown
// names so a typoed --addons flag fails instead of silently thinning the
// repo.
func selected(a Answers) ([]Component, error) {
	ids := []string{"core"}
	for _, n := range a.AddOns {
		ids = append(ids, "addon:"+n)
	}
	for _, n := range a.Agents {
		ids = append(ids, "agent:"+n)
	}
	if a.SecurityKeys {
		ids = append(ids, "feature:security-keys")
	}

	var components []Component
	for _, id := range ids {
		i := slices.IndexFunc(manifest, func(c Component) bool { return c.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("unknown component %s", id)
		}
		components = append(components, manifest[i])
	}
	return components, nil
}
