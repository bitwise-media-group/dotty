// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// call is one recorded invocation: the kind of runner method, the child's
// extra environment, and the argv (binary first).
type call struct {
	kind string // "run", "output", or "interactive"
	env  []string
	argv []string
}

// fakeRunner satisfies InstallRunner, recording every invocation and
// answering from canned outputs keyed by the joined argv.
type fakeRunner struct {
	calls   []call
	outputs map[string][]byte // argv (joined by space, binary excluded) → stdout
	errs    map[string]error  // argv → error
}

func (f *fakeRunner) record(kind string, env []string, name string, args []string) error {
	f.calls = append(f.calls, call{kind: kind, env: env, argv: append([]string{name}, args...)})
	return f.errs[strings.Join(args, " ")]
}

func (f *fakeRunner) RunEnv(_ context.Context, env []string, name string, args ...string) error {
	return f.record("run", env, name, args)
}

func (f *fakeRunner) OutputEnv(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
	err := f.record("output", env, name, args)
	return f.outputs[strings.Join(args, " ")], err
}

func (f *fakeRunner) RunInteractiveEnv(_ context.Context, env []string, name string, args ...string) error {
	return f.record("interactive", env, name, args)
}

// argvs renders the recorded calls as "<kind> <args…>" strings for
// comparison, checking on the way that every call named the client's
// binary and carried the profile's MISE_CONFIG_DIR.
func (f *fakeRunner) argvs(t *testing.T, bin, dir string) []string {
	t.Helper()
	var out []string
	for _, c := range f.calls {
		if c.argv[0] != bin {
			t.Errorf("call %v ran %q, want %q", c.argv, c.argv[0], bin)
		}
		if want := "MISE_CONFIG_DIR=" + dir; !slices.Contains(c.env, want) {
			t.Errorf("call %v env = %v, want %s", c.argv, c.env, want)
		}
		out = append(out, c.kind+" "+strings.Join(c.argv[1:], " "))
	}
	return out
}

// profileDir lays down a mise directory with the given config.toml and
// conf.d fragments (name → content) and returns it.
func profileDir(t *testing.T, config string, fragments map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "mise")
	if err := os.MkdirAll(ConfDDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if config != "" {
		if err := os.WriteFile(ConfigPath(dir), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range fragments {
		if err := os.WriteFile(filepath.Join(ConfDDir(dir), name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const testConfig = `[settings]
lockfile = true

[tools]
"aqua:sharkdp/fd" = "latest"

[bootstrap.packages]
"brew:git" = "latest"
"brew-cask:ghostty" = { version = "latest", os = "macos" }
`

var testFragments = map[string]string{
	"core.toml": "[tools]\n\"aqua:jqlang/jq\" = \"latest\"\n\n[bootstrap.packages]\n\"brew:curl\" = \"latest\"\n",
}

func wantCalls(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("calls =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestAddDispatchesToolsAndPackages pins the split: tool ids go through
// `mise use`, package ids through `mise bootstrap packages use`, each
// global, non-interactive-confirmed, and against the profile's directory;
// adding a tool re-locks every platform.
func TestAddDispatchesToolsAndPackages(t *testing.T) {
	dir := profileDir(t, testConfig, testFragments)
	r := &fakeRunner{}
	c := New(r, "/usr/local/bin/mise", dir)

	res, err := c.Add(context.Background(), []string{"aqua:sharkdp/bat", "brew:wget", "github:o/r", "brew-cask:raycast"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("skipped = %v, want none", res.Skipped)
	}
	wantCalls(t, r.argvs(t, "/usr/local/bin/mise", dir), []string{
		"interactive use --global --yes aqua:sharkdp/bat github:o/r",
		"run lock --global",
		"interactive bootstrap packages use --global --yes brew:wget brew-cask:raycast",
	})
}

// TestAddSkipsDeclared pins the dedupe: ids declared in config.toml or a
// fragment are reported as skipped, repeats within one call collapse, and
// an all-skipped add still installs the profile.
func TestAddSkipsDeclared(t *testing.T) {
	dir := profileDir(t, testConfig, testFragments)

	t.Run("partial", func(t *testing.T) {
		r := &fakeRunner{}
		c := New(r, "mise", dir)
		res, err := c.Add(context.Background(), []string{"aqua:sharkdp/fd", "aqua:jqlang/jq", "brew:curl",
			"brew:wget", "brew:wget"})
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		// The repeated wget is skipped too: its first mention declared it.
		if want := []string{"aqua:sharkdp/fd", "aqua:jqlang/jq", "brew:curl", "brew:wget"}; !slices.Equal(res.Skipped, want) {
			t.Errorf("skipped = %v, want %v", res.Skipped, want)
		}
		wantCalls(t, r.argvs(t, "mise", dir), []string{
			"interactive bootstrap packages use --global --yes brew:wget",
		})
	})

	t.Run("all skipped installs", func(t *testing.T) {
		r := &fakeRunner{}
		c := New(r, "mise", dir)
		res, err := c.Add(context.Background(), []string{"brew:git"})
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if !slices.Equal(res.Skipped, []string{"brew:git"}) {
			t.Errorf("skipped = %v", res.Skipped)
		}
		wantCalls(t, r.argvs(t, "mise", dir), []string{
			"run lock --global",
			"interactive install --yes",
			"interactive bootstrap packages apply --yes",
		})
	})
}

// TestRemoveRefusesFragmentEntries pins the ownership split: entries a
// conf.d fragment declares belong to dotty init and are reported as
// managed; unknown ids as not found; neither reaches mise.
func TestRemoveRefusesFragmentEntries(t *testing.T) {
	dir := profileDir(t, testConfig, testFragments)
	r := &fakeRunner{}
	c := New(r, "mise", dir)

	res, err := c.Remove(context.Background(), []string{"aqua:jqlang/jq", "brew:curl", "brew:nope", "aqua:sharkdp/fd"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if want := []string{"aqua:jqlang/jq", "brew:curl"}; !slices.Equal(res.Managed, want) {
		t.Errorf("managed = %v, want %v", res.Managed, want)
	}
	if want := []string{"brew:nope"}; !slices.Equal(res.NotFound, want) {
		t.Errorf("not found = %v, want %v", res.NotFound, want)
	}
	wantCalls(t, r.argvs(t, "mise", dir), []string{"run unuse --global aqua:sharkdp/fd"})
}

// TestRemovePackagesEditsConfig pins that bootstrap packages leave through
// a file edit — mise has no unuse for them — with the rest of config.toml
// intact and nothing run.
func TestRemovePackagesEditsConfig(t *testing.T) {
	dir := profileDir(t, testConfig, testFragments)
	r := &fakeRunner{}
	c := New(r, "mise", dir)

	res, err := c.Remove(context.Background(), []string{"brew-cask:ghostty"})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(res.Managed) != 0 || len(res.NotFound) != 0 {
		t.Errorf("result = %+v, want clean", res)
	}
	if len(r.calls) != 0 {
		t.Errorf("calls = %v, want none for a package removal", r.calls)
	}
	got, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "ghostty") {
		t.Errorf("entry survived:\n%s", got)
	}
	for _, keep := range []string{"lockfile = true", `"aqua:sharkdp/fd" = "latest"`, `"brew:git" = "latest"`} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("missing %q after remove:\n%s", keep, got)
		}
	}
}

const pruneListing = "mise bootstrap packages prune\nremove brew:mise@2026.9.1\nremove brew:bat@0.26.1\n"

// TestSync pins the argv sequence per flag and confirmation outcome.
func TestSync(t *testing.T) {
	const install = "interactive install --yes"
	const lock = "run lock --global"
	const pruneTools = "run prune --yes"
	const apply = "interactive bootstrap packages apply --yes"
	const dryRun = "output bootstrap packages prune --dry-run"
	const prunePackages = "interactive bootstrap packages prune --yes"

	cases := []struct {
		name         string
		force        bool
		listing      string
		dryRunErr    error
		answer       bool
		wantRemovals []string
		wantCalls    []string
		wantErr      bool
	}{
		{
			name: "force skips the dry run and prunes", force: true,
			wantCalls: []string{lock, install, pruneTools, apply, prunePackages},
		},
		{
			name: "nothing to remove skips the package prune", listing: "",
			wantCalls: []string{dryRun, lock, install, pruneTools, apply},
		},
		{
			name: "confirmed removals prune", listing: pruneListing, answer: true,
			wantRemovals: []string{"brew:mise@2026.9.1", "brew:bat@0.26.1"},
			wantCalls:    []string{dryRun, lock, install, pruneTools, apply, prunePackages},
		},
		{
			name: "declined removals abort", listing: pruneListing, answer: false,
			wantRemovals: []string{"brew:mise@2026.9.1", "brew:bat@0.26.1"},
			wantCalls:    []string{dryRun},
		},
		{
			name: "dry run failure is an error", dryRunErr: errors.New("boom"),
			wantCalls: []string{dryRun}, wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := profileDir(t, testConfig, nil)
			r := &fakeRunner{
				outputs: map[string][]byte{"bootstrap packages prune --dry-run": []byte(c.listing)},
				errs:    map[string]error{"bootstrap packages prune --dry-run": c.dryRunErr},
			}
			var gotRemovals []string
			confirm := func(removals []string) (bool, error) {
				gotRemovals = removals
				return c.answer, nil
			}
			err := New(r, "mise", dir).Sync(context.Background(), c.force, confirm)
			if (err != nil) != c.wantErr {
				t.Fatalf("Sync error = %v, wantErr %v", err, c.wantErr)
			}
			if !slices.Equal(gotRemovals, c.wantRemovals) {
				t.Errorf("confirm got %v, want %v", gotRemovals, c.wantRemovals)
			}
			wantCalls(t, r.argvs(t, "mise", dir), c.wantCalls)
		})
	}
}

func TestUpgrade(t *testing.T) {
	dir := profileDir(t, testConfig, nil)
	r := &fakeRunner{}
	if err := New(r, "mise", dir).Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	wantCalls(t, r.argvs(t, "mise", dir), []string{
		"interactive upgrade --yes",
		"run lock --global",
		"interactive bootstrap packages upgrade --yes",
	})
}

func TestStatusLockImport(t *testing.T) {
	dir := profileDir(t, testConfig, nil)
	r := &fakeRunner{}
	c := New(r, "mise", dir)
	ctx := context.Background()
	if err := c.Status(ctx); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if err := c.Lock(ctx); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := c.Import(ctx, "/x/config.toml", true); err != nil {
		t.Fatalf("Import: %v", err)
	}
	wantCalls(t, r.argvs(t, "mise", dir), []string{
		"run ls --global",
		"run bootstrap packages status",
		"run lock --global",
		"run bootstrap packages import --manager brew --path /x/config.toml --all",
	})
}

// importRunner answers `bootstrap packages import` by writing canned
// content to the --path target, the way mise does.
type importRunner struct {
	fakeRunner
	content string
}

func (f *importRunner) RunEnv(ctx context.Context, env []string, name string, args ...string) error {
	if err := f.fakeRunner.RunEnv(ctx, env, name, args...); err != nil {
		return err
	}
	if i := slices.Index(args, "--path"); i >= 0 && i+1 < len(args) {
		return os.WriteFile(args[i+1], []byte(f.content), 0o644)
	}
	return nil
}

// TestImportToScratch pins that the import never targets the real config,
// leaves no scratch file behind, and hands back the package and tap entries.
func TestImportToScratch(t *testing.T) {
	dir := profileDir(t, testConfig, nil)
	r := &importRunner{content: "[bootstrap.brew.taps]\n\"acme/tap\" = \"https://github.com/acme/homebrew-tap.git\"\n\n" +
		"[bootstrap.packages]\n\"brew:acme/tap/widget\" = \"latest\"\n\"brew:wget\" = \"latest\"\n"}
	entries, err := ImportToScratch(context.Background(), r, "mise", dir, false)
	if err != nil {
		t.Fatalf("ImportToScratch: %v", err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Table+" "+e.ID)
	}
	want := []string{"bootstrap.brew.taps acme/tap", "bootstrap.packages brew:acme/tap/widget", "bootstrap.packages brew:wget"}
	if !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
	config, err := os.ReadFile(ConfigPath(dir))
	if err != nil || string(config) != testConfig {
		t.Errorf("config.toml changed by the import: %v\n%s", err, config)
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".import-*"))
	if len(files) != 0 {
		t.Errorf("scratch files left behind: %v", files)
	}
}

func TestIsPackageID(t *testing.T) {
	cases := map[string]bool{
		"brew:git": true, "brew-cask:ghostty": true, "mas:497799835": true, "flatpak:org.x": true,
		"apt:curl": true, "aqua:sharkdp/bat": false, "github:o/r": false, "go": false, "npm:x": false,
	}
	for id, want := range cases {
		if got := IsPackageID(id); got != want {
			t.Errorf("IsPackageID(%q) = %v, want %v", id, got, want)
		}
	}
	tools := map[string]bool{"aqua:sharkdp/bat": true, "go": true, "node": true, "npm:x": true,
		"brew:git": false, "git": false, "flux": false}
	for id, want := range tools {
		if got := IsToolID(id); got != want {
			t.Errorf("IsToolID(%q) = %v, want %v", id, got, want)
		}
	}
}
