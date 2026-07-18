// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// everything selects every component the manifest knows.
func everything() Answers {
	return Answers{
		ProfileName:  "test",
		ReposDir:     "/repos",
		AddOns:       []string{"nvim", "btop", "k9s", "lazygit", "lsd", "tmux", "yazi"},
		Agents:       []string{"claude-code", "codex", "opencode", "antigravity", "grok"},
		SecurityKeys: true,
	}
}

// TestManifestMatchesEmbed catches drift in both directions: manifest entries
// pointing at nothing, and embedded files no component claims.
func TestManifestMatchesEmbed(t *testing.T) {
	claimed := map[string]bool{sharedDoc: true}
	for _, c := range manifest {
		for _, prefix := range c.Prefixes {
			if _, err := fs.Stat(templateFS, prefix); err != nil {
				t.Errorf("component %s: prefix %s not embedded: %v", c.ID, prefix, err)
				continue
			}
			_ = fs.WalkDir(templateFS, prefix, func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					claimed[path] = true
				}
				return nil
			})
		}
		for src := range c.Renames {
			if _, err := fs.Stat(templateFS, src); err != nil {
				t.Errorf("component %s: rename source %s not embedded: %v", c.ID, src, err)
			}
			claimed[src] = true
		}
		if c.Brewfile != "" {
			if _, err := fs.Stat(templateFS, c.Brewfile); err != nil {
				t.Errorf("component %s: brewfile %s not embedded: %v", c.ID, c.Brewfile, err)
			}
			claimed[c.Brewfile] = true
		}
	}

	_ = fs.WalkDir(templateFS, "template", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !claimed[path] {
			t.Errorf("embedded file %s is not claimed by any component", path)
		}
		return nil
	})

	sets := map[string]map[string]bool{"templated": templated, "executable": executable, "per-profile": perProfile}
	for name, set := range sets {
		for path := range set {
			if _, err := fs.Stat(templateFS, path); err != nil {
				t.Errorf("%s entry %s not embedded: %v", name, path, err)
			}
		}
	}
}

func TestPlanDestinationsAreUnique(t *testing.T) {
	ops, err := Plan(everything())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	seen := make(map[string]bool)
	for _, op := range ops {
		if seen[op.Dst] {
			t.Errorf("destination %s produced twice", op.Dst)
		}
		seen[op.Dst] = true
	}
}

func TestPlanRejectsUnknownSelection(t *testing.T) {
	if _, err := Plan(Answers{AddOns: []string{"emacs"}}); err == nil {
		t.Fatal("Plan accepted an unknown add-on")
	}
}

func TestRenderEverything(t *testing.T) {
	for _, marketplace := range []bool{false, true} {
		a := everything()
		a.Marketplace = marketplace
		a.Harden = marketplace // exercise both toggles across the two passes

		ops, err := Plan(a)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		repo, machineDir := t.TempDir(), t.TempDir()
		v := NewVars(a, "/home/x", repo)
		if err := Render(ops, repo, machineDir, v); err != nil {
			t.Fatalf("Render(marketplace=%v): %v", marketplace, err)
		}

		assertRenderedOps(t, ops, repo, machineDir)

		// The strict-JSON agent config must parse in both marketplace branches,
		// and only carry the bitwise marketplace when asked for.
		data, err := os.ReadFile(filepath.Join(machineDir, "home/.config/claude/settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		var settings map[string]any
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatalf("claude settings.json (marketplace=%v) is not valid JSON: %v", marketplace, err)
		}
		if _, has := settings["extraKnownMarketplaces"]; has != marketplace {
			t.Errorf("extraKnownMarketplaces present=%v, want %v", has, marketplace)
		}
		grok, err := os.ReadFile(filepath.Join(machineDir, "home/.config/grok/config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(grok, []byte(`name = "bitwise"`)) != marketplace {
			t.Errorf("grok bitwise marketplace present != %v", marketplace)
		}

		assertHardenToggles(t, machineDir, settings, grok, marketplace)

		// The machine env carries the per-machine values the repo no longer does.
		env, err := os.ReadFile(filepath.Join(machineDir, "env.zsh"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`REPOS_DIR="/repos"`, "EDITOR=nvim", "CODEX_HOME"} {
			if !bytes.Contains(env, []byte(want)) {
				t.Errorf("env.zsh missing %s:\n%s", want, env)
			}
		}
		gitcfg, err := os.ReadFile(filepath.Join(machineDir, "git.gitconfig"))
		if err != nil || !bytes.Contains(gitcfg, []byte("format = ssh")) {
			t.Errorf("git.gitconfig missing signing block (securityKeys=true): %v", err)
		}
	}
}

// assertRenderedOps checks every planned op landed in the right root with
// the right mode, no template residue, and no machine-varying bytes in the
// shared repository.
func assertRenderedOps(t *testing.T, ops []FileOp, repo, machineDir string) {
	t.Helper()
	for _, op := range ops {
		root := repo
		if op.PerProfile {
			root = machineDir
		}
		path := filepath.Join(root, op.Dst)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("missing output %s: %v", op.Dst, err)
		}
		if op.LinkTo != "" {
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s: want symlink", op.Dst)
			}
			continue
		}
		if op.Mode == 0o755 && info.Mode().Perm() != 0o755 {
			t.Errorf("%s: mode %o, want 755", op.Dst, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if op.Templated && bytes.Contains(data, []byte("{{")) {
			t.Errorf("%s: unrendered template residue", op.Dst)
		}
		if bytes.Contains(data, []byte("/Users/deavon")) {
			t.Errorf("%s: machine-specific path leaked", op.Dst)
		}
		if !op.PerProfile && bytes.Contains(data, []byte("/repos")) {
			t.Errorf("%s: ReposDir rendered into the shared repository", op.Dst)
		}
	}
}

// assertHardenToggles checks that hardening gates each agent's confinement
// in its native config.
func assertHardenToggles(t *testing.T, machineDir string, settings map[string]any, grok []byte, harden bool) {
	t.Helper()
	if _, has := settings["sandbox"]; has != harden {
		t.Errorf("claude sandbox present=%v, want %v", has, harden)
	}
	if bytes.Contains(grok, []byte("\nprofile = \"dotfiles\"")) != harden {
		t.Errorf("grok sandbox profile present != %v", harden)
	}
	codex, err := os.ReadFile(filepath.Join(machineDir, "home/.config/codex/config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(codex, []byte("\n[sandbox_workspace_write]")) != harden {
		t.Errorf("codex sandbox_workspace_write present != %v", harden)
	}
	if bytes.Contains(codex, []byte("hooks.PreToolUse")) != harden {
		t.Errorf("codex PreToolUse hook present != %v", harden)
	}
	if !bytes.Contains(codex, []byte("approval_policy")) {
		t.Error("codex approval_policy missing in both branches")
	}
	opencode, err := os.ReadFile(filepath.Join(machineDir, "home/.config/opencode/opencode.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(opencode, []byte(`"permission"`)) != harden {
		t.Errorf("opencode permission block present != %v", harden)
	}
}

func TestRenderSharedDocLinks(t *testing.T) {
	a := Answers{ProfileName: "t", ReposDir: "/r", Agents: []string{"codex", "grok", "claude-code"}}
	ops, err := Plan(a)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	repo := t.TempDir()
	if err := Render(ops, repo, t.TempDir(), NewVars(a, "/home/x", repo)); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// claude is primary; the others resolve to its file through relative links.
	primary, err := os.ReadFile(filepath.Join(repo, "home/.config/claude/CLAUDE.md"))
	if err != nil {
		t.Fatalf("primary doc: %v", err)
	}
	for _, doc := range []string{"home/.config/codex/AGENTS.md", "home/.config/grok/AGENTS.md"} {
		info, err := os.Lstat(filepath.Join(repo, doc))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s: want symlink, got %v, %v", doc, info, err)
		}
		got, err := os.ReadFile(filepath.Join(repo, doc))
		if err != nil || !bytes.Equal(got, primary) {
			t.Fatalf("%s does not resolve to the primary doc: %v", doc, err)
		}
	}
}

// TestRenderWorktreesShapes pins the two worktree layouts: the repo-relative
// default rides the ReposDir sandbox grants and lands in the shared
// gitignore; an absolute root becomes an explicit sandbox path and skips the
// ignore.
func TestRenderWorktreesShapes(t *testing.T) {
	render := func(t *testing.T, worktrees string) (repo, machine string) {
		t.Helper()
		a := everything()
		a.Worktrees = worktrees
		a.Harden = true // the sandbox grants live in the hardened blocks
		ops, err := Plan(a)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		repo, machine = t.TempDir(), t.TempDir()
		if err := Render(ops, repo, machine, NewVars(a, "/home/x", repo)); err != nil {
			t.Fatalf("Render: %v", err)
		}
		return repo, machine
	}
	read := func(t *testing.T, root, rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	t.Run("relative default", func(t *testing.T) {
		repo, machine := render(t, "")
		got := read(t, machine, "worktrees.gitconfig")
		if !strings.Contains(got, "gitdir:**/.git/worktrees/**") || strings.Contains(got, "includeIf \"gitdir:/") {
			t.Errorf("worktrees.gitconfig:\n%s", got)
		}
		if got := read(t, repo, "home/.config/git/ignore"); !strings.Contains(got, "\n.worktrees/\n") {
			t.Errorf("gitignore missing worktree dir:\n%s", got)
		}
		codexCfg := read(t, machine, "home/.config/codex/config.toml")
		if strings.Contains(codexCfg, `".worktrees`) || strings.Contains(codexCfg, "agent/worktrees") {
			t.Error("codex writable_roots should not carry a relative worktree path")
		}
		if got := read(t, machine, "home/.config/claude/settings.json"); !strings.Contains(got, `"Edit(**/.worktrees/**)"`) {
			t.Errorf("claude Edit rule missing:\n%.400s", got)
		}
		opencodeCfg := read(t, machine, "home/.config/opencode/opencode.jsonc")
		if !strings.Contains(opencodeCfg, `"**/.worktrees": "allow"`) {
			t.Error("opencode matrix missing worktree allow")
		}
		if got := read(t, machine, "env.zsh"); !strings.Contains(got, `DOTTY_WORKTREES=".worktrees"`) {
			t.Errorf("env.zsh missing DOTTY_WORKTREES:\n%s", got)
		}
	})

	t.Run("absolute root", func(t *testing.T) {
		repo, machine := render(t, "~/.cache/agent/worktrees")
		abs := "/home/x/.cache/agent/worktrees"
		got := read(t, machine, "worktrees.gitconfig")
		if !strings.Contains(got, "gitdir:"+abs+"/**") || !strings.Contains(got, "gitdir:**/.git/worktrees/**") {
			t.Errorf("worktrees.gitconfig:\n%s", got)
		}
		if got := read(t, repo, "home/.config/git/ignore"); strings.Contains(got, "worktrees") {
			t.Error("gitignore should not list an absolute worktree root")
		}
		if got := read(t, machine, "home/.config/codex/config.toml"); !strings.Contains(got, `"`+abs+`",`) {
			t.Error("codex writable_roots missing the absolute root")
		}
		if got := read(t, machine, "home/.config/claude/settings.json"); !strings.Contains(got, `"`+abs+`",`) {
			t.Error("claude allowWrite missing the absolute root")
		}
	})
}

// TestPrunePerProfileRemovesOrphans pins the re-render cleanup: renders left
// at destinations the plan no longer produces — a template relocation like
// ~/.claude/settings.json → ~/.config/claude/settings.json, or a deselected
// agent — are removed along with the directories they emptied, while planned
// renders survive.
func TestPrunePerProfileRemovesOrphans(t *testing.T) {
	a := everything()
	ops, err := Plan(a)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	repo, profileDir := t.TempDir(), t.TempDir()
	if err := Render(ops, repo, profileDir, NewVars(a, "/home/x", repo)); err != nil {
		t.Fatalf("Render: %v", err)
	}

	orphans := []string{
		filepath.Join("home", ".claude", "settings.json"), // pre-XDG location
		filepath.Join("home", ".grok", "sandbox.toml"),
	}
	for _, rel := range orphans {
		path := filepath.Join(profileDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stale render"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pruned, err := PrunePerProfile(profileDir, ops)
	if err != nil {
		t.Fatalf("PrunePerProfile: %v", err)
	}
	slices.Sort(pruned)
	want := []string{".claude/settings.json", ".grok/sandbox.toml"}
	if !slices.Equal(pruned, want) {
		t.Errorf("pruned = %v, want %v", pruned, want)
	}
	for _, gone := range []string{
		filepath.Join(profileDir, "home", ".claude"), // emptied directory goes too
		filepath.Join(profileDir, "home", ".grok"),
	} {
		if _, err := os.Lstat(gone); err == nil {
			t.Errorf("%s survived the prune", gone)
		}
	}
	for _, op := range ops {
		if !op.PerProfile || !strings.HasPrefix(op.Dst, "home/") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(profileDir, op.Dst)); err != nil {
			t.Errorf("planned render %s pruned: %v", op.Dst, err)
		}
	}

	// A second pass over the now-clean tree is a no-op, and a profile with no
	// home tree yet prunes nothing.
	if pruned, err := PrunePerProfile(profileDir, ops); err != nil || len(pruned) != 0 {
		t.Errorf("re-prune = %v, %v; want none", pruned, err)
	}
	if pruned, err := PrunePerProfile(t.TempDir(), ops); err != nil || len(pruned) != 0 {
		t.Errorf("prune without a home tree = %v, %v; want none", pruned, err)
	}
}

func TestComposeBrewfileDedupes(t *testing.T) {
	a := everything()
	composed, err := ComposeBrewfile(a)
	if err != nil {
		t.Fatalf("ComposeBrewfile: %v", err)
	}
	seen := make(map[string]int)
	for line := range strings.Lines(string(composed)) {
		trimmed := strings.TrimRight(line, "\n")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		seen[trimmed]++
	}
	for line, n := range seen {
		if n > 1 {
			t.Errorf("line %q appears %d times", line, n)
		}
	}
	if !strings.Contains(string(composed), `brew "lazygit"`) {
		t.Error("composed Brewfile is missing lazygit (nvim and lazygit both pull it)")
	}
	// dotty manages the machine, so every profile's Brewfile installs it.
	for _, line := range []string{`tap "bitwise-media-group/tap"`, `cask "bitwise-media-group/tap/dotty"`} {
		if !strings.Contains(string(composed), line) {
			t.Errorf("composed Brewfile is missing %s", line)
		}
	}
}

func TestMergeBrewfile(t *testing.T) {
	existing := []byte("brew \"git\"\nbrew \"lazygit\"\n")
	composed := []byte("# core\nbrew \"git\"\nbrew \"tmux\"\n")
	merged := string(MergeBrewfile(existing, composed))
	if strings.Count(merged, `brew "git"`) != 1 {
		t.Errorf("git duplicated:\n%s", merged)
	}
	if !strings.Contains(merged, `brew "tmux"`) {
		t.Errorf("tmux missing:\n%s", merged)
	}
}

func TestAnswersRoundTrip(t *testing.T) {
	repo := t.TempDir()
	want := everything()
	if err := SaveAnswers(repo, want); err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}
	got, err := LoadAnswers(repo)
	if err != nil {
		t.Fatalf("LoadAnswers: %v", err)
	}
	if got.ProfileName != want.ProfileName || len(got.AddOns) != len(want.AddOns) ||
		len(got.Agents) != len(want.Agents) || got.SecurityKeys != want.SecurityKeys {
		t.Fatalf("round trip mismatch: %+v != %+v", got, want)
	}
}

// TestAnswerKeysSurviveFalsyAnswers is the --yes contract: a saved "no" or an
// answered-empty pick counts as answered after the round trip, while slices
// that were never collected (nil) do not.
func TestAnswerKeysSurviveFalsyAnswers(t *testing.T) {
	dir := t.TempDir()
	a := Answers{
		ProfileName: "test",
		ReposDir:    "~/Repos",
		AddOns:      []string{},
		Agents:      []string{},
		// Harden, DumpBrews, PIV, Wallpaper stay zero: answered "no"/"keep".
		// MacOSDefaults and AllowedSerials stay nil: never asked.
	}
	if err := SaveAnswers(dir, a); err != nil {
		t.Fatalf("SaveAnswers: %v", err)
	}
	_, keys, err := LoadAnswersWithKeys(dir)
	if err != nil {
		t.Fatalf("LoadAnswersWithKeys: %v", err)
	}
	answered := []struct {
		name string
		got  bool
	}{
		{"ReposDir", keys.ReposDir}, {"Repo", keys.Repo},
		{"AddOns", keys.AddOns}, {"Agents", keys.Agents},
		{"DumpBrews", keys.DumpBrews}, {"SecurityKeys", keys.SecurityKeys},
		{"Harden", keys.Harden}, {"Marketplace", keys.Marketplace},
		{"Wallpaper", keys.Wallpaper}, {"PIV", keys.PIV},
	}
	for _, k := range answered {
		if !k.got {
			t.Errorf("key %s: answered falsy value not reported as answered", k.name)
		}
	}
	if keys.MacOSDefaults {
		t.Error("key MacOSDefaults: nil slice reported as answered")
	}
}

// TestAnswerKeysAbsentInLegacyDocument pins the other half of the contract: a
// document written before a question existed — or carrying an explicit null —
// does not answer it, so a --yes re-run still asks.
func TestAnswerKeysAbsentInLegacyDocument(t *testing.T) {
	dir := t.TempDir()
	doc := `{"profile":"test","reposDir":"~/Repos","addons":["tmux"],"macosDefaults":null}`
	if err := os.WriteFile(filepath.Join(dir, AnswersFile), []byte(doc), 0o644); err != nil {
		t.Fatalf("write %s: %v", AnswersFile, err)
	}
	got, keys, err := LoadAnswersWithKeys(dir)
	if err != nil {
		t.Fatalf("LoadAnswersWithKeys: %v", err)
	}
	if !keys.ReposDir || !keys.AddOns {
		t.Errorf("stored keys not reported as answered: %+v", keys)
	}
	if keys.Harden || keys.SecurityKeys || keys.Wallpaper || keys.PIV {
		t.Errorf("missing keys reported as answered: %+v", keys)
	}
	if keys.MacOSDefaults {
		t.Error("key MacOSDefaults: null value reported as answered")
	}
	if !slices.Equal(got.AddOns, []string{"tmux"}) {
		t.Errorf("answers not loaded alongside keys: %+v", got)
	}
}

func TestUnfoldCoversAgentStateDirs(t *testing.T) {
	dirs, err := Unfold(everything())
	if err != nil {
		t.Fatalf("Unfold: %v", err)
	}
	for _, want := range []string{".config", ".config/claude", ".config/grok", ".config/codex", ".ssh"} {
		if !slices.Contains(dirs, want) {
			t.Errorf("Unfold missing %s (got %v)", want, dirs)
		}
	}
}
