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

// Fnox is the slice of *fnox.Client the migrator writes through.
type Fnox interface {
	Set(ctx context.Context, t fnox.Target, provider, key string, value []byte) error
	SetDefault(ctx context.Context, t fnox.Target, key, def string) error
	Keys(ctx context.Context, t fnox.Target) ([]string, error)
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

// Migrator moves credentials from the legacy store into fnox.
type Migrator struct {
	ios   cli.IOStreams
	store *env.Store
	fnox  Fnox
}

// New returns a migrator reading store and writing through f.
func New(ios cli.IOStreams, store *env.Store, f Fnox) *Migrator {
	return &Migrator{ios: ios, store: store, fnox: f}
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

// Files migrates .env.dotty templates: each becomes (or extends) the
// fnox.toml in its directory, literals as defaults and references as values
// resolved from the keychain. The template is left in place for the user to
// delete once fnox exec works.
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
	dst := filepath.Join(filepath.Dir(path), ProjectConfig)
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

	for _, line := range env.Lines(string(src)) {
		if line.Err != nil {
			tui.Warnf(m.ios, "%s:%d: %v", path, line.N, line.Err)
			report.Failed++
			continue
		}
		if !opts.Force && slices.Contains(existing, line.Key) {
			report.Skipped++
			continue
		}
		if opts.DryRun {
			tui.Infof(m.ios, "would set %s in %s", line.Key, t)
			report.Migrated++
			continue
		}
		if line.Ref == nil {
			if err := m.fnox.SetDefault(ctx, t, line.Key, line.Value); err != nil {
				tui.Warnf(m.ios, "%s:%d: %v", path, line.N, err)
				report.Failed++
				continue
			}
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
	if !opts.DryRun {
		tui.Infof(m.ios, "Wrote %s; once `fnox exec` works there, delete %s and run `fnox check --all`", dst, path)
	}
	return report, nil
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
