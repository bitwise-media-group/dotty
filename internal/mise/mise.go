// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Runner executes mise on behalf of this package; tests substitute a fake.
// Every call carries the environment that retargets mise's global config
// directory at the profile.
type Runner interface {
	RunEnv(ctx context.Context, extraEnv []string, name string, args ...string) error
	OutputEnv(ctx context.Context, extraEnv []string, name string, args ...string) ([]byte, error)
}

// InstallRunner is Runner plus the terminal-attached variant the install
// entry points need: mise prompts (cask installers may ask for sudo), and
// its progress output wants a tty.
type InstallRunner interface {
	Runner
	RunInteractiveEnv(ctx context.Context, extraEnv []string, name string, args ...string) error
}

// packageManagers are the `[bootstrap.packages]` managers mise knows; an id
// carrying one of these prefixes is a bootstrap package, everything else a
// tool.
var packageManagers = []string{"brew", "brew-cask", "apt", "apk", "dnf", "pacman", "mas", "flatpak", "flatpak-user"}

// IsPackageID reports whether id names a bootstrap package (`brew:git`,
// `brew-cask:ghostty`, `mas:497799835`) rather than a tool.
func IsPackageID(id string) bool {
	manager, _, ok := strings.Cut(id, ":")
	return ok && slices.Contains(packageManagers, manager)
}

// IsToolID reports whether id is a full backend id (`aqua:sharkdp/bat`,
// `github:o/r`, `npm:x`) or one of mise's core tools (`go`, `node`,
// `python`, …). Registry shorthand — `git`, `flux` — is deliberately not a
// tool id here: it resolves to whichever registry entry claims the name
// (git-chglog, flux-operator), so callers make the user spell the backend.
func IsToolID(id string) bool {
	if IsPackageID(id) {
		return false
	}
	if strings.Contains(id, ":") {
		return true
	}
	return slices.Contains(coreTools, id)
}

// coreTools are the backends mise implements itself, usable bare.
var coreTools = []string{"bun", "deno", "dotnet", "elixir", "erlang", "go", "java", "node", "python", "ruby",
	"rust", "swift", "zig"}

// Client runs mise verbs against one profile's mise directory.
type Client struct {
	Bin string // the mise binary; "mise" resolves through PATH
	Dir string // the profile's mise directory (config.toml, conf.d/, mise.lock)
	r   InstallRunner
}

// New returns a client driving bin against the mise directory dir. An empty
// bin resolves "mise" through PATH.
func New(r InstallRunner, bin, dir string) *Client {
	if bin == "" {
		bin = "mise"
	}
	return &Client{Bin: bin, Dir: dir, r: r}
}

// env is the environment every invocation carries: the profile's mise
// directory as the global config dir, so `--global` verbs and the lockfile
// land in the profile.
func (c *Client) env() []string {
	return []string{"MISE_CONFIG_DIR=" + c.Dir}
}

func (c *Client) run(ctx context.Context, args ...string) error {
	return c.r.RunEnv(ctx, c.env(), c.Bin, args...)
}

func (c *Client) output(ctx context.Context, args ...string) ([]byte, error) {
	return c.r.OutputEnv(ctx, c.env(), c.Bin, args...)
}

func (c *Client) interactive(ctx context.Context, args ...string) error {
	return c.r.RunInteractiveEnv(ctx, c.env(), c.Bin, args...)
}

// AddResult reports what Add skipped: ids the profile already declares.
type AddResult struct {
	Skipped []string
}

// Add records ids in the profile's config.toml and installs them: tools via
// `mise use --global`, bootstrap packages via `mise bootstrap packages use
// --global`. Ids already declared anywhere in the profile — config.toml or
// a conf.d fragment — are skipped rather than duplicated. When every id was
// skipped the profile is installed anyway, converging a machine where an
// entry is recorded but not installed. Adding a tool refreshes mise.lock for
// every platform, so the commit that adds the tool carries its lock.
func (c *Client) Add(ctx context.Context, ids []string) (AddResult, error) {
	var res AddResult
	declared, err := Declared(c.Dir)
	if err != nil {
		return res, err
	}
	var tools, packages []string
	for _, id := range ids {
		if _, ok := declared[id]; ok {
			res.Skipped = append(res.Skipped, id)
			continue
		}
		declared[id] = Entry{} // also dedupes repeats within one invocation
		if IsPackageID(id) {
			packages = append(packages, id)
		} else {
			tools = append(tools, id)
		}
	}
	if len(tools) == 0 && len(packages) == 0 {
		return res, c.Install(ctx)
	}
	if len(tools) > 0 {
		if err := c.interactive(ctx, append([]string{"use", "--global", "--yes"}, tools...)...); err != nil {
			return res, err
		}
		if err := c.Lock(ctx); err != nil {
			return res, err
		}
	}
	if len(packages) > 0 {
		args := append([]string{"bootstrap", "packages", "use", "--global", "--yes"}, packages...)
		if err := c.interactive(ctx, args...); err != nil {
			return res, err
		}
	}
	return res, nil
}

// RemoveResult reports what Remove could not do: NotFound ids are not
// declared anywhere in the profile; Managed ids are declared by a conf.d
// fragment, which dotty init owns — deselect the component instead.
type RemoveResult struct {
	NotFound []string
	Managed  []string
}

// Remove drops ids from the profile's config.toml: tools via `mise unuse
// --global`, bootstrap packages by editing the file (mise has no
// `bootstrap packages unuse`). Nothing is uninstalled — removed entries stay
// on the machine until Sync.
func (c *Client) Remove(ctx context.Context, ids []string) (RemoveResult, error) {
	var res RemoveResult
	declared, err := Declared(c.Dir)
	if err != nil {
		return res, err
	}
	var tools, packages []string
	for _, id := range ids {
		entry, ok := declared[id]
		switch {
		case !ok:
			res.NotFound = append(res.NotFound, id)
			continue
		case entry.File != ConfigPath(c.Dir):
			res.Managed = append(res.Managed, id)
			continue
		}
		delete(declared, id) // also dedupes repeats within one invocation
		if IsPackageID(id) {
			packages = append(packages, id)
		} else {
			tools = append(tools, id)
		}
	}
	if len(tools) > 0 {
		if err := c.run(ctx, append([]string{"unuse", "--global"}, tools...)...); err != nil {
			return res, err
		}
	}
	for _, id := range packages {
		if err := RemoveEntry(ConfigPath(c.Dir), id); err != nil {
			return res, err
		}
	}
	return res, nil
}

// Sync makes the machine match the profile: lock the tools for every
// platform, install them and the bootstrap packages, then prune what the
// profile no longer declares. Locking comes first so a tool a fragment
// added since the last sync — or a profile seeded with a lockfile that
// predates it — installs from a resolved, checksummed entry rather than
// failing the lockfile check. Unless force is set, the bootstrap-package
// removals are listed first (a prune dry run) and confirm decides whether to
// proceed; returning false aborts with no changes. Tool versions the profile
// no longer references are always pruned — they are mise-owned installs,
// invisible to the rest of the machine.
func (c *Client) Sync(ctx context.Context, force bool, confirm func(removals []string) (bool, error)) error {
	prunePackages := force
	if !force {
		removals, err := c.pruneDryRun(ctx)
		if err != nil {
			return err
		}
		if len(removals) > 0 {
			ok, err := confirm(removals)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			prunePackages = true
		}
	}
	if err := c.Lock(ctx); err != nil {
		return err
	}
	if err := c.interactive(ctx, "install", "--yes"); err != nil {
		return err
	}
	if err := c.run(ctx, "prune", "--yes"); err != nil {
		return err
	}
	if err := c.interactive(ctx, "bootstrap", "packages", "apply", "--yes"); err != nil {
		return err
	}
	if !prunePackages {
		return nil
	}
	return c.interactive(ctx, "bootstrap", "packages", "prune", "--yes")
}

// pruneDryRun lists the bootstrap packages a prune would remove, one
// `remove <manager>:<name>@<version>` line each.
func (c *Client) pruneDryRun(ctx context.Context) ([]string, error) {
	out, err := c.output(ctx, "bootstrap", "packages", "prune", "--dry-run")
	if err != nil {
		return nil, fmt.Errorf("check for removable packages: %w", err)
	}
	var removals []string
	for line := range strings.Lines(string(out)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "remove "); ok {
			removals = append(removals, rest)
		}
	}
	return removals, nil
}

// Upgrade moves every tool and bootstrap package to its newest version
// without removing anything, and refreshes mise.lock for every platform.
func (c *Client) Upgrade(ctx context.Context) error {
	if err := c.interactive(ctx, "upgrade", "--yes"); err != nil {
		return err
	}
	if err := c.Lock(ctx); err != nil {
		return err
	}
	return c.interactive(ctx, "bootstrap", "packages", "upgrade", "--yes")
}

// Install installs whatever the profile declares that is missing: the tools,
// locked first, then the bootstrap packages.
func (c *Client) Install(ctx context.Context) error {
	if err := c.Lock(ctx); err != nil {
		return err
	}
	if err := c.interactive(ctx, "install", "--yes"); err != nil {
		return err
	}
	return c.interactive(ctx, "bootstrap", "packages", "apply", "--yes")
}

// Status prints the tools (`mise ls --global`) and the bootstrap packages
// (`mise bootstrap packages status`) with their installed state.
func (c *Client) Status(ctx context.Context) error {
	if err := c.run(ctx, "ls", "--global"); err != nil {
		return err
	}
	return c.run(ctx, "bootstrap", "packages", "status")
}

// Lock refreshes mise.lock for every platform the profile's tools publish —
// the one committed lockfile serves macOS and Linux — creating it when the
// profile has none yet.
func (c *Client) Lock(ctx context.Context) error {
	return c.run(ctx, "lock", "--global")
}

// Import records the Homebrew formulae installed on this machine as
// `brew:` bootstrap packages in the config file at path — the formulae
// installed on request, or with all every linked formula including
// dependencies.
func (c *Client) Import(ctx context.Context, path string, all bool) error {
	args := []string{"bootstrap", "packages", "import", "--manager", "brew", "--path", path}
	if all {
		args = append(args, "--all")
	}
	return c.run(ctx, args...)
}

// ImportToScratch runs the Homebrew import (see Import; all includes
// dependencies) into a scratch config beside the profile's config.toml in
// dir and returns the entries it produced — the `[bootstrap.packages]` declarations and the
// `[bootstrap.brew.taps]` the tapped ones need; the real config is never the
// import target, so a re-run cannot disturb entries the import does not
// cover. It takes a plain Runner because the scaffold calls it while
// rendering, before any interactive install.
func ImportToScratch(ctx context.Context, r Runner, bin, dir string, all bool) ([]Entry, error) {
	if bin == "" {
		bin = "mise"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".import-*.toml")
	if err != nil {
		return nil, fmt.Errorf("create scratch import file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close scratch import file: %w", err)
	}
	args := []string{"bootstrap", "packages", "import", "--manager", "brew", "--path", tmpPath}
	if all {
		args = append(args, "--all")
	}
	if err := r.RunEnv(ctx, []string{"MISE_CONFIG_DIR=" + dir}, bin, args...); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read scratch import file: %w", err)
	}
	var entries []Entry
	for _, e := range scanEntries(data, filepath.Base(tmpPath)) {
		if e.Table == TablePackages || e.Table == TableTaps {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// ErrNotInstalled reports that no usable mise binary was found and none
// could be installed.
var ErrNotInstalled = errors.New("mise is not installed")
