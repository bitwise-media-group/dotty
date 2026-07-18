// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
}

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
	return FileOp{Src: src, Dst: dst, Mode: mode, Templated: templated[src], PerProfile: perProfile[src]}
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

// PrunePerProfile removes entries under the profile's home/ tree that ops no
// longer render — orphans left behind when a template file relocates
// (~/.claude/settings.json → ~/.config/claude/settings.json) or a component
// is deselected. The tree is wholly machine-rendered, so an unplanned entry
// is never a user file; directories the pruning empties go too. It returns
// the pruned paths relative to the home/ tree, so the caller can heal the
// live symlinks they leave dangling.
func PrunePerProfile(profileDir string, ops []FileOp) ([]string, error) {
	planned := make(map[string]bool, len(ops))
	for _, op := range ops {
		if op.PerProfile {
			planned[op.Dst] = true
		}
	}

	root := filepath.Join(profileDir, "home")
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
		return nil, nil // no home tree yet — nothing to prune
	}
	if err != nil {
		return nil, fmt.Errorf("prune profile renders: %w", err)
	}

	var pruned []string
	for _, path := range orphans {
		if err := os.Remove(path); err != nil {
			return pruned, fmt.Errorf("prune %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return pruned, err
		}
		pruned = append(pruned, rel)
		for dir := filepath.Dir(path); dir != root; dir = filepath.Dir(dir) {
			if os.Remove(dir) != nil {
				break // still holds planned renders; so do its parents
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

// ComposeBrewfile concatenates the core Brewfile fragment with each
// selection's, dropping lines already emitted so shared dependencies appear
// once.
func ComposeBrewfile(a Answers) ([]byte, error) {
	components, err := selected(a)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	seen := make(map[string]bool)
	for _, c := range components {
		if c.Brewfile == "" {
			continue
		}
		data, err := fs.ReadFile(templateFS, c.Brewfile)
		if err != nil {
			return nil, fmt.Errorf("component %s: %w", c.ID, err)
		}
		for line := range strings.Lines(string(data)) {
			trimmed := strings.TrimRight(line, "\n")
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				if seen[trimmed] {
					continue
				}
				seen[trimmed] = true
			}
			out.WriteString(trimmed)
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

// MergeBrewfile appends composed's package lines to an existing Brewfile
// (typically a fresh `brew bundle dump`), skipping lines the dump already
// carries and leaving comments and blanks as structure.
func MergeBrewfile(existing, composed []byte) []byte {
	present := make(map[string]bool)
	for line := range strings.Lines(string(existing)) {
		present[strings.TrimRight(line, "\n")] = true
	}
	out := bytes.TrimRight(existing, "\n")
	var extra []string
	for line := range strings.Lines(string(composed)) {
		trimmed := strings.TrimRight(line, "\n")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || present[trimmed] {
			continue
		}
		present[trimmed] = true
		extra = append(extra, trimmed)
	}
	if len(extra) == 0 {
		return append(out, '\n')
	}
	out = append(out, "\n\n# template packages\n"...)
	out = append(out, strings.Join(extra, "\n")...)
	return append(out, '\n')
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
