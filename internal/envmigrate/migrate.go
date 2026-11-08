// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package envmigrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/fnox"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// DefaultNamespace is the legacy namespace the old verbs used when none was
// given; it maps onto fnox's top-level secrets rather than a profile.
const DefaultNamespace = "default"

// ProjectConfig is the per-directory fnox config file a template becomes.
const ProjectConfig = "fnox.toml"

// projectHeader opens a fnox.toml dotty creates. fnox refuses to write
// through -c to a file that does not exist, so the file is seeded first —
// with the secrets table already present, so fnox's edit keeps the comment
// above it rather than trailing the entries it adds.
const projectHeader = "# Written by dotty env migrate from .env.dotty; edit freely.\n\n[secrets]\n"

// Fnox is the slice of *fnox.Client the migrator writes secrets through.
type Fnox interface {
	Set(ctx context.Context, t fnox.Target, provider, key string, value []byte) error
	Keys(ctx context.Context, t fnox.Target) ([]string, error)
}

// Env is the slice of *mise.ProjectEnv the migrator writes plain variables
// through.
type Env interface {
	Keys(path string) ([]string, error)
	Set(ctx context.Context, path, key, value string) error
	Trust(ctx context.Context, path string) error
}

// Options selects what to migrate and how. Empty Namespaces means every
// namespace in the keychain; Files are .env.dotty template paths. Force
// overwrites keys fnox already has, Purge deletes a keychain namespace once
// it migrated with no failures, DryRun reports without writing, and Provider
// is the fnox provider that encrypts the values.
type Options struct {
	Namespaces []string
	Files      []string
	Force      bool
	Purge      bool
	DryRun     bool
	Provider   string
}

// Report counts what a run did. Purged lists the keychain namespaces removed.
type Report struct {
	Migrated int
	Skipped  int
	Failed   int
	Purged   []string
}

// Add folds o into r.
func (r *Report) Add(o Report) {
	r.Migrated += o.Migrated
	r.Skipped += o.Skipped
	r.Failed += o.Failed
	r.Purged = append(r.Purged, o.Purged...)
}

// Migrator moves credentials from the legacy store into fnox, and plain
// template variables into mise.
type Migrator struct {
	ios   cli.IOStreams
	store *env.Store
	fnox  Fnox
	env   Env
}

// New returns a migrator reading store, writing secrets through f and plain
// variables through e.
func New(ios cli.IOStreams, store *env.Store, f Fnox, e Env) *Migrator {
	return &Migrator{ios: ios, store: store, fnox: f, env: e}
}

// Target maps a legacy namespace onto its fnox home in the global config.
func Target(namespace string) fnox.Target {
	if namespace == DefaultNamespace {
		return fnox.Target{Global: true}
	}
	return fnox.Target{Global: true, Profile: namespace}
}

// Namespaces migrates keychain namespaces (all of them when opts names
// none). A namespace's failures are warned about and counted; they never
// abort the run, but they do withhold that namespace's purge.
func (m *Migrator) Namespaces(ctx context.Context, opts Options) (Report, error) {
	namespaces := opts.Namespaces
	if len(namespaces) == 0 {
		var err error
		if namespaces, err = m.store.Namespaces(ctx); err != nil {
			return Report{}, err
		}
	}
	if len(namespaces) == 0 {
		tui.Infof(m.ios, "No legacy credentials in the keychain")
		return Report{}, nil
	}

	var report Report
	for _, ns := range namespaces {
		if err := env.ValidateNamespace(ns); err != nil {
			tui.Warnf(m.ios, "skipping namespace: %v", err)
			report.Failed++
			continue
		}
		r, failed := m.namespace(ctx, opts, ns)
		report.Add(r)
		if !opts.Purge || opts.DryRun {
			continue
		}
		if failed {
			tui.Warnf(m.ios, "keeping keychain namespace %q: not every credential migrated", ns)
			continue
		}
		if err := m.store.DeleteNamespace(ctx, ns); err != nil {
			tui.Warnf(m.ios, "purge namespace %q: %v", ns, err)
			continue
		}
		report.Purged = append(report.Purged, ns)
	}
	return report, nil
}

// namespace migrates one namespace and reports whether anything failed.
func (m *Migrator) namespace(ctx context.Context, opts Options, ns string) (Report, bool) {
	var report Report
	t := Target(ns)
	values, err := m.store.All(ctx, ns)
	if err != nil {
		tui.Warnf(m.ios, "read namespace %q: %v", ns, err)
		report.Failed++
		return report, true
	}
	if len(values) == 0 {
		tui.Infof(m.ios, "Namespace %q has no credentials", ns)
		return report, false
	}
	existing, err := m.existingKeys(ctx, opts, t)
	if err != nil {
		tui.Warnf(m.ios, "%v", err)
		report.Failed++
		return report, true
	}

	keys := slices.Sorted(maps.Keys(values))
	for _, key := range keys {
		if !opts.Force && slices.Contains(existing, key) {
			report.Skipped++
			continue
		}
		value := values[key]
		if value != strings.TrimSpace(value) {
			tui.Warnf(m.ios, "%s/%s: fnox trims surrounding whitespace; the stored value will differ", ns, key)
		}
		if opts.DryRun {
			tui.Infof(m.ios, "would set %s in %s", key, t)
			report.Migrated++
			continue
		}
		if err := m.fnox.Set(ctx, t, opts.Provider, key, []byte(value)); err != nil {
			tui.Warnf(m.ios, "%v", err)
			report.Failed++
			continue
		}
		report.Migrated++
	}
	return report, report.Failed > 0
}

// Files migrates .env.dotty templates. Plain KEY=value lines become
// variables in the mise.toml beside the template — they are configuration,
// not secrets, and mise loads them for the shell and its tasks alike — while
// references become encrypted values in the fnox.toml beside it, which is
// only created when the template has any. The template is left in place for
// the user to delete once both work.
func (m *Migrator) Files(ctx context.Context, opts Options) (Report, error) {
	var report Report
	for _, path := range opts.Files {
		r, err := m.file(ctx, opts, path)
		if err != nil {
			return report, err
		}
		report.Add(r)
	}
	return report, nil
}

// file migrates one template.
func (m *Migrator) file(ctx context.Context, opts Options, path string) (Report, error) {
	var report Report
	src, err := os.ReadFile(path)
	if err != nil {
		return report, fmt.Errorf("read template: %w", err)
	}
	var plain, refs []env.Line
	for _, line := range env.Lines(string(src)) {
		switch {
		case line.Err != nil:
			tui.Warnf(m.ios, "%s:%d: %v", path, line.N, line.Err)
			report.Failed++
		case line.Ref == nil:
			plain = append(plain, line)
		default:
			refs = append(refs, line)
		}
	}

	dir := filepath.Dir(path)
	var wrote []string
	if len(plain) > 0 {
		dst := filepath.Join(dir, mise.ProjectConfig)
		r, err := m.plain(ctx, opts, path, dst, plain)
		report.Add(r)
		if err != nil {
			return report, err
		}
		wrote = append(wrote, dst)
	}
	if len(refs) > 0 {
		dst := filepath.Join(dir, ProjectConfig)
		r, err := m.secrets(ctx, opts, path, dst, refs)
		report.Add(r)
		if err != nil {
			return report, err
		}
		wrote = append(wrote, dst)
	}
	if !opts.DryRun && len(wrote) > 0 {
		tui.Infof(m.ios, "Wrote %s; once they work, delete %s%s", strings.Join(wrote, " and "), path,
			checkHint(len(refs) > 0))
	}
	return report, nil
}

// checkHint is the follow-up for a template that had secrets.
func checkHint(secrets bool) string {
	if !secrets {
		return ""
	}
	return " and run `fnox check --all`"
}

// plain carries a template's literal assignments into the mise config at
// dst. mise loads a project file only once it is trusted, so a file this
// run creates is trusted straight away; one that already existed keeps
// whatever trust the user gave it.
func (m *Migrator) plain(ctx context.Context, opts Options, path, dst string, lines []env.Line) (Report, error) {
	var report Report
	existing, err := m.existingEnv(opts, dst)
	if err != nil {
		return report, err
	}
	_, statErr := os.Stat(dst)
	created := errors.Is(statErr, fs.ErrNotExist)

	for _, line := range lines {
		if !opts.Force && slices.Contains(existing, line.Key) {
			report.Skipped++
			continue
		}
		if mise.Expands(line.Value) {
			tui.Warnf(m.ios, "%s:%d: mise expands $ and {{ }} in %s; check the value in %s", path, line.N, line.Key, dst)
		}
		if opts.DryRun {
			tui.Infof(m.ios, "would set %s in %s", line.Key, dst)
			report.Migrated++
			continue
		}
		if err := m.env.Set(ctx, dst, line.Key, line.Value); err != nil {
			tui.Warnf(m.ios, "%s:%d: %v", path, line.N, err)
			report.Failed++
			continue
		}
		report.Migrated++
	}
	if created && !opts.DryRun && report.Migrated > 0 {
		if err := m.env.Trust(ctx, dst); err != nil {
			tui.Warnf(m.ios, "%v (run mise trust %s)", err, dst)
		}
	}
	return report, nil
}

// secrets carries a template's references into the fnox config at dst,
// resolving each from the keychain.
func (m *Migrator) secrets(ctx context.Context, opts Options, path, dst string, lines []env.Line) (Report, error) {
	var report Report
	t := fnox.Target{Config: dst}
	if !opts.DryRun {
		if err := ensureProjectConfig(dst); err != nil {
			return report, err
		}
	}
	existing, err := m.existingKeys(ctx, opts, t)
	if err != nil {
		return report, err
	}

	for _, line := range lines {
		if !opts.Force && slices.Contains(existing, line.Key) {
			report.Skipped++
			continue
		}
		if opts.DryRun {
			tui.Infof(m.ios, "would set %s in %s", line.Key, t)
			report.Migrated++
			continue
		}
		ns := line.Ref.Namespace
		if ns == "" {
			ns = DefaultNamespace
		}
		value, err := m.store.Get(ctx, ns, line.Ref.Key)
		if err != nil {
			tui.Warnf(m.ios, "%s:%d: %v", path, line.N, err)
			report.Failed++
			continue
		}
		if err := m.fnox.Set(ctx, t, opts.Provider, line.Key, []byte(value)); err != nil {
			tui.Warnf(m.ios, "%s:%d: %v", path, line.N, err)
			report.Failed++
			continue
		}
		report.Migrated++
	}
	return report, nil
}

// existingEnv lists the variables the mise config at dst already declares so
// re-runs skip them.
func (m *Migrator) existingEnv(opts Options, dst string) ([]string, error) {
	if opts.Force {
		return nil, nil
	}
	return m.env.Keys(dst)
}

// existingKeys lists what t already holds so re-runs skip it. A project file
// that does not exist yet (dry run) holds nothing.
func (m *Migrator) existingKeys(ctx context.Context, opts Options, t fnox.Target) ([]string, error) {
	if opts.Force {
		return nil, nil
	}
	if !t.Global {
		if _, err := os.Stat(t.Config); errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
	}
	return m.fnox.Keys(ctx, t)
}

// ensureProjectConfig seeds dst when it does not exist.
func ensureProjectConfig(dst string) error {
	_, err := os.Stat(dst)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("stat %s: %w", dst, err)
	}
	return cli.AtomicWriteFile(dst, []byte(projectHeader), 0o644)
}
