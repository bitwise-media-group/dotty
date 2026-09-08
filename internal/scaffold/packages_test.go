// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
)

// TestPlanRendersMiseFragments pins that every selected component's mise
// fragment lands as a per-profile render under mise/conf.d, alongside the
// keep-existing user config.
func TestPlanRendersMiseFragments(t *testing.T) {
	a := Answers{ProfileName: "t", ReposDir: "/r", AddOns: []string{"tmux", "nvim"}, Agents: []string{"codex"},
		SecurityKeys: true}
	ops, err := Plan(a)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var fragments []string
	config := false
	for _, op := range ops {
		if op.Dst == "mise/config.toml" {
			config = op.PerProfile && op.KeepExisting
		}
		if rest, ok := strings.CutPrefix(op.Dst, MiseFragmentsDir+"/"); ok {
			if !op.PerProfile {
				t.Errorf("%s is not per-profile", op.Dst)
			}
			fragments = append(fragments, rest)
		}
	}
	want := []string{"addon-nvim.toml", "addon-tmux.toml", "agent-codex.toml", "core.toml", "feature-security-keys.toml"}
	if !slices.Equal(fragments, want) {
		t.Errorf("fragments = %v, want %v", fragments, want)
	}
	if !config {
		t.Error("mise/config.toml missing, or not a per-profile keep-existing render")
	}
}

// TestRenderKeepsExistingConfig pins the ownership split: the first render
// seeds mise/config.toml from the template, a re-render leaves the user's
// edits alone, and the fragments are re-rendered regardless.
func TestRenderKeepsExistingConfig(t *testing.T) {
	a := Answers{ProfileName: "t", ReposDir: "/r", AddOns: []string{"tmux"}}
	ops, err := Plan(a)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	repo, profileDir := t.TempDir(), t.TempDir()
	if err := Render(ops, repo, profileDir, NewVars(a, "/home/x", repo)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	configPath := filepath.Join(profileDir, "mise", "config.toml")
	got, err := os.ReadFile(configPath)
	if err != nil || string(got) != mise.DefaultConfig {
		t.Fatalf("first render config = %q, %v; want the template", got, err)
	}
	edited := mise.DefaultConfig + "\"brew:wget\" = \"latest\"\n"
	if err := os.WriteFile(configPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	fragment := filepath.Join(profileDir, MiseFragmentsDir, "addon-tmux.toml")
	if err := os.WriteFile(fragment, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Render(ops, repo, profileDir, NewVars(a, "/home/x", repo)); err != nil {
		t.Fatalf("re-render: %v", err)
	}
	if got, _ := os.ReadFile(configPath); string(got) != edited {
		t.Errorf("re-render reset the user config:\n%s", got)
	}
	if got, _ := os.ReadFile(fragment); !strings.Contains(string(got), "tmux-builds") {
		t.Errorf("fragment not re-rendered:\n%s", got)
	}
}

// TestPrunePerProfileCoversConfD pins that a deselected component's
// fragment is pruned while the user's config.toml and lockfile survive.
func TestPrunePerProfileCoversConfD(t *testing.T) {
	a := Answers{ProfileName: "t", ReposDir: "/r", AddOns: []string{"tmux"}}
	ops, err := Plan(a)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	repo, profileDir := t.TempDir(), t.TempDir()
	if err := Render(ops, repo, profileDir, NewVars(a, "/home/x", repo)); err != nil {
		t.Fatalf("Render: %v", err)
	}
	stale := filepath.Join(profileDir, MiseFragmentsDir, "addon-lsd.toml")
	if err := os.WriteFile(stale, []byte("[tools]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(profileDir, "mise", "mise.lock")
	if err := os.WriteFile(lock, []byte("lockfile_version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pruned, err := PrunePerProfile(profileDir, ops)
	if err != nil {
		t.Fatalf("PrunePerProfile: %v", err)
	}
	if !slices.Equal(pruned, []string{filepath.Join(MiseFragmentsDir, "addon-lsd.toml")}) {
		t.Errorf("pruned = %v", pruned)
	}
	for _, keep := range []string{lock, filepath.Join(profileDir, "mise", "config.toml"),
		filepath.Join(profileDir, MiseFragmentsDir, "addon-tmux.toml")} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s pruned: %v", keep, err)
		}
	}
}

// TestSeedMiseConfig pins the takeover of a machine's existing global mise
// config: copied into the profile with dotty's settings added, lockfile
// included, only when the profile has none yet.
func TestSeedMiseConfig(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	live := filepath.Join(xdg, "mise")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	liveConfig := "[settings]\nlocked = false\n\n[tools]\nnode = \"lts\"\n\"aqua:anchore/quill\" = \"latest\"\n"
	if err := os.WriteFile(filepath.Join(live, "config.toml"), []byte(liveConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(live, "mise.lock"), []byte("lockfile_version = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}

	miseDir := filepath.Join(t.TempDir(), "mise")
	if err := seedMiseConfig(ios, miseDir); err != nil {
		t.Fatalf("seedMiseConfig: %v", err)
	}
	got, err := os.ReadFile(mise.ConfigPath(miseDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"node = \"lts\"", "\"aqua:anchore/quill\" = \"latest\"",
		"[settings]\nlocked = false\n# dotty: lock every tool\nlockfile = true",
		"[bootstrap.brew]\n# dotty: keep the casks already installed\nadopt = true"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("seeded config missing %q:\n%s", want, got)
		}
	}
	if strings.Count(string(got), "lockfile = true") != 1 {
		t.Errorf("lockfile setting duplicated:\n%s", got)
	}
	if _, err := os.Stat(mise.LockPath(miseDir)); err != nil {
		t.Errorf("lockfile not copied: %v", err)
	}

	// Seeding never overwrites a profile that has a config already.
	if err := os.WriteFile(mise.ConfigPath(miseDir), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seedMiseConfig(ios, miseDir); err != nil {
		t.Fatalf("seedMiseConfig again: %v", err)
	}
	if got, _ := os.ReadFile(mise.ConfigPath(miseDir)); string(got) != "mine" {
		t.Errorf("existing profile config overwritten: %q", got)
	}

	// A live directory that is already a symlink (the profile link) is
	// not a seed source.
	if err := os.RemoveAll(live); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(miseDir, live); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(t.TempDir(), "mise")
	if err := seedMiseConfig(ios, fresh); err != nil {
		t.Fatalf("seedMiseConfig from link: %v", err)
	}
	if _, err := os.Stat(mise.ConfigPath(fresh)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("seeded from the profile link: %v", err)
	}
}

// importRunner answers `mise bootstrap packages import` by writing canned
// content to the --path target, the way mise does.
type importRunner struct {
	content string
	err     error
	calls   int
}

func (f *importRunner) RunEnv(_ context.Context, _ []string, _ string, args ...string) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	if i := slices.Index(args, "--path"); i >= 0 {
		return os.WriteFile(args[i+1], []byte(f.content), 0o644)
	}
	return nil
}

func (f *importRunner) OutputEnv(context.Context, []string, string, ...string) ([]byte, error) {
	return nil, f.err
}

// composeForTest runs composeProfilePackages against a profile whose mise
// directory holds config (the template default when empty) and the
// fragments of answers a, with a fake mise on PATH so the import can run,
// and returns the resulting config.toml plus the warning stream.
func composeForTest(t *testing.T, config, brewfile string, r *importRunner, a Answers) (string, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	bin := mise.LocalBin(home)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	profileDir := t.TempDir()
	if config == "" {
		config = mise.DefaultConfig
	}
	if err := os.MkdirAll(mise.Dir(profileDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mise.ConfigPath(mise.Dir(profileDir)), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if brewfile != "" {
		if err := os.WriteFile(filepath.Join(profileDir, "Brewfile"), []byte(brewfile), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ops, err := Plan(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := Render(ops, t.TempDir(), profileDir, NewVars(a, home, "/repo")); err != nil {
		t.Fatal(err)
	}
	errOut := &bytes.Buffer{}
	ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: errOut}
	if err := composeProfilePackages(context.Background(), ios, r, profileDir, a, home); err != nil {
		t.Fatalf("composeProfilePackages: %v", err)
	}
	got, err := os.ReadFile(mise.ConfigPath(mise.Dir(profileDir)))
	if err != nil {
		t.Fatal(err)
	}
	if scratch, _ := filepath.Glob(filepath.Join(mise.Dir(profileDir), ".import-*")); len(scratch) > 0 {
		t.Errorf("scratch import files left behind: %v", scratch)
	}
	return string(got), errOut
}

// TestImportPackagesMerges pins the seed: imported formulae land under
// their header, entries the fragments or the user already declare are
// dropped, taps ride along, and a failed import warns instead of failing.
func TestImportPackagesMerges(t *testing.T) {
	answers := Answers{ProfileName: "box", AddOns: []string{"tmux"}, ImportPackages: true}
	imported := "[bootstrap.brew.taps]\n\"acme/tap\" = \"https://github.com/acme/homebrew-tap.git\"\n\n" +
		"[bootstrap.packages]\n\"brew:acme/tap/widget\" = \"latest\"\n\"brew:git\" = \"latest\"\n\"brew:wget\" = \"latest\"\n"

	t.Run("merges new entries only", func(t *testing.T) {
		config := mise.DefaultConfig + "\"brew:wget\" = \"latest\"\n"
		got, _ := composeForTest(t, config, "", &importRunner{content: imported}, answers)
		if !strings.Contains(got, "# installed packages\n\"brew:acme/tap/widget\" = \"latest\"\n") {
			t.Errorf("imported package missing:\n%s", got)
		}
		if strings.Count(got, `"brew:wget"`) != 1 || strings.Contains(got, `"brew:git"`) {
			t.Errorf("user or fragment entry duplicated:\n%s", got)
		}
		if !strings.Contains(got, "[bootstrap.brew.taps]\n# installed packages\n\"acme/tap\"") {
			t.Errorf("tap missing:\n%s", got)
		}
	})

	t.Run("re-run is idempotent", func(t *testing.T) {
		got, _ := composeForTest(t, "", "", &importRunner{content: imported}, answers)
		home := t.TempDir()
		r := &importRunner{content: imported}
		// Second pass over the same config: nothing new, file unchanged.
		profileDir := t.TempDir()
		if err := os.MkdirAll(mise.Dir(profileDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mise.ConfigPath(mise.Dir(profileDir)), []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
		if err := composeProfilePackages(context.Background(), ios, r, profileDir, answers, home); err != nil {
			t.Fatal(err)
		}
		if r.calls != 0 {
			t.Errorf("import ran without mise on this home")
		}
		if again, _ := os.ReadFile(mise.ConfigPath(mise.Dir(profileDir))); string(again) != got {
			t.Errorf("second pass changed the config:\n%s", again)
		}
	})

	t.Run("import failure warns", func(t *testing.T) {
		got, errOut := composeForTest(t, "", "", &importRunner{err: errors.New("no brew")}, answers)
		if got != mise.DefaultConfig {
			t.Errorf("config changed by a failed import:\n%s", got)
		}
		if !strings.Contains(errOut.String(), "dotty packages import") {
			t.Errorf("no warning for the failed import: %q", errOut.String())
		}
	})

	t.Run("not asked", func(t *testing.T) {
		r := &importRunner{content: imported}
		a := answers
		a.ImportPackages = false
		if got, _ := composeForTest(t, "", "", r, a); got != mise.DefaultConfig || r.calls != 0 {
			t.Errorf("import ran unasked (%d calls):\n%s", r.calls, got)
		}
	})
}

// TestConvertProfileBrewfile pins the migration of a profile from before
// packages moved to mise: template formulae the fragments now cover are
// dropped, the rest land in config.toml under their header, the Brewfile
// stays in place, and a second pass is silent.
func TestConvertProfileBrewfile(t *testing.T) {
	answers := Answers{ProfileName: "box", AddOns: []string{"tmux"}}
	brewfile := "tap \"acme/tap\", trusted: true\nbrew \"tmux\"\nbrew \"git\"\nbrew \"colima\"\nbrew \"acme/tap/widget\"\n" +
		"cask \"raycast\"\ncask \"bitwise-media-group/tap/evolve\", trusted: true\nvscode \"golang.go\"\n"
	got, errOut := composeForTest(t, "", brewfile, &importRunner{}, answers)
	for _, want := range []string{
		"[tools]\n# imported from Brewfile\n\"github:bitwise-media-group/evolve\" = \"latest\"\n",
		"# imported from Brewfile\n\"brew:colima\" = \"latest\"\n\"brew:acme/tap/widget\" = \"latest\"\n" +
			"\"brew-cask:raycast\" = { version = \"latest\", os = \"macos\" }\n",
		"[bootstrap.brew.taps]\n# imported from Brewfile\n\"acme/tap\" = \"https://github.com/acme/homebrew-tap.git\"\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("converted config missing %q:\n%s", want, got)
		}
	}
	for _, gone := range []string{"tmux", `"brew:git"`} {
		if strings.Contains(got, gone) {
			t.Errorf("fragment-covered entry %s converted anyway:\n%s", gone, got)
		}
	}
	if !strings.Contains(errOut.String(), `vscode "golang.go"`) {
		t.Errorf("warning for the vscode entry missing: %q", errOut.String())
	}
}

// TestConverterMapMatchesFragments keeps the converter honest: every tool it
// maps a template-era formula or agent cask onto is declared by some
// fragment, so a converted Brewfile entry is always covered by a component
// rather than silently invented.
func TestConverterMapMatchesFragments(t *testing.T) {
	declared := make(map[string]bool)
	for _, c := range manifest {
		if c.Mise == "" {
			continue
		}
		data, err := fs.ReadFile(templateFS, c.Mise)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(data)) {
			if id, _, ok := strings.Cut(strings.TrimPrefix(line, `"`), `"`); ok {
				declared[id] = true
			}
		}
	}
	for _, id := range mise.FragmentToolIDs() {
		if !declared[id] {
			t.Errorf("converter maps onto %s, which no fragment declares", id)
		}
	}
}

// TestConfigTemplateMatchesDefault pins that the template's config.toml and
// the mise package's DefaultConfig (used for hand-made profiles) agree.
func TestConfigTemplateMatchesDefault(t *testing.T) {
	data, err := fs.ReadFile(templateFS, "template/mise/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != mise.DefaultConfig {
		t.Errorf("template/mise/config.toml differs from mise.DefaultConfig:\n%s", data)
	}
}

// TestAnswersLegacyImportKey pins that a profile answered under the
// Brewfile-era key still counts as answered, value included.
func TestAnswersLegacyImportKey(t *testing.T) {
	dir := t.TempDir()
	doc := `{"profile":"test","reposDir":"~/Repos","dumpBrews":true}`
	if err := os.WriteFile(filepath.Join(dir, AnswersFile), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	got, keys, err := LoadAnswersWithKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !keys.ImportPackages || !got.ImportPackages {
		t.Errorf("legacy dumpBrews not honoured: keys=%v answers=%+v", keys.ImportPackages, got)
	}
}
