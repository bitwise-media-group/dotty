// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/signingkey"
	"github.com/bitwise-media-group/dotty/internal/wizard"
)

// initEnv points HOME and the XDG dirs at scratch space and returns the fake
// home. Buffered IOStreams keep every prompt non-interactive, so flags decide
// everything.
func initEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return home
}

func TestInitEndToEnd(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"),
		"--profile-name=testbox", "--addons=tmux,lsd", "--agents=claude-code,codex",
		"--dump-brews=false", "--marketplace", "--on-conflict=backup", "--yes", "--skip-font", "--skip-git")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	assertRepoAndProfile(t, home, repo)
	assertHomeLinks(t, home, repo)

	// Idempotent re-run: answers reload, nothing conflicts.
	if err := execDotty(t, "init", "--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"),
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run: %v", err)
	}
}

// assertRepoAndProfile checks the rendered repository and the repo-hosted
// profile after the TestInitEndToEnd init run.
func assertRepoAndProfile(t *testing.T, home, repo string) {
	t.Helper()
	// The answers live in the profile — they are this machine's, not the
	// shared repository's.
	profileDir := filepath.Join(home, ".config", "dotty", "testbox")
	answers, err := scaffold.LoadAnswers(profileDir)
	if err != nil {
		t.Fatalf("answers not persisted in profile: %v", err)
	}
	if answers.ProfileName != "testbox" || len(answers.AddOns) != 2 || len(answers.Agents) != 2 || !answers.Marketplace {
		t.Fatalf("persisted answers = %+v", answers)
	}
	for _, want := range []string{
		"home/.config/ghostty/config",
		"home/.config/tmux/tmux.conf",
		"home/.config/claude/CLAUDE.md",
		"CREDITS.md",
	} {
		if _, err := os.Stat(filepath.Join(repo, want)); err != nil {
			t.Errorf("repo missing %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "home/.config/yazi")); err == nil {
		t.Error("unselected add-on yazi was rendered")
	}

	// Machine-varying files render into the profile, never the shared repo.
	for _, machineFile := range []string{"home/.config/claude/settings.json", "home/.config/codex/dotty.config.toml"} {
		if _, err := os.Stat(filepath.Join(repo, machineFile)); err == nil {
			t.Errorf("%s rendered into the shared repository", machineFile)
		}
		if _, err := os.Stat(filepath.Join(profileDir, machineFile)); err != nil {
			t.Errorf("%s missing from profile: %v", machineFile, err)
		}
	}
	for _, f := range []string{"env.zsh", "git.gitconfig"} {
		if _, err := os.Stat(filepath.Join(profileDir, f)); err != nil {
			t.Errorf("profile missing %s: %v", f, err)
		}
	}

	// The profile exists, is active, and its Brewfile carries the selections.
	brew, err := os.ReadFile(filepath.Join(profileDir, "Brewfile"))
	if err != nil {
		t.Fatalf("profile Brewfile: %v", err)
	}
	if !containsLine(string(brew), `brew "tmux"`) {
		t.Errorf("Brewfile missing tmux:\n%s", brew)
	}
	active, err := os.Readlink(filepath.Join(home, ".config", "dotty", "active-profile"))
	if err != nil || filepath.Base(active) != "testbox" {
		t.Errorf("active-profile = %q, %v", active, err)
	}
}

// assertHomeLinks checks $HOME after linking: folded links where nothing
// existed, real unfold dirs preserved, machine files linked through
// active-profile so activation swaps them.
func assertHomeLinks(t *testing.T, home, repo string) {
	t.Helper()
	wantTmux := filepath.Join(repo, "home", ".config", "tmux")
	if link, err := os.Readlink(filepath.Join(home, ".config", "tmux")); err != nil || link != wantTmux {
		t.Errorf("~/.config/tmux link = %q, %v", link, err)
	}
	for _, real := range []string{".config/claude", ".config/codex", ".config/zsh"} {
		info, err := os.Lstat(filepath.Join(home, real))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			t.Errorf("~/%s should be a real directory: %v, %v", real, info, err)
		}
	}
	wantDest := filepath.Join(home, ".config", "dotty", "active-profile",
		"home", ".config", "claude", "settings.json")
	claudeSettings := filepath.Join(home, ".config", "claude", "settings.json")
	if link, err := os.Readlink(claudeSettings); err != nil || link != wantDest {
		t.Errorf("~/.claude/settings.json link = %q, %v (want %s)", link, err, wantDest)
	}
	if got, err := os.ReadFile(claudeSettings); err != nil || len(got) == 0 {
		t.Errorf("machine link does not resolve through active-profile: %v", err)
	}
	wantCodex := filepath.Join(home, ".config", "dotty", "active-profile",
		"home", ".config", "codex", "dotty.config.toml")
	codexLayer := filepath.Join(home, ".config", "codex", "dotty.config.toml")
	if link, err := os.Readlink(codexLayer); err != nil || link != wantCodex {
		t.Errorf("~/.config/codex/dotty.config.toml link = %q, %v (want %s)", link, err, wantCodex)
	}
	if got, err := os.ReadFile(codexLayer); err != nil || len(got) == 0 {
		t.Errorf("codex profile layer does not resolve through active-profile: %v", err)
	}
}

// TestInitRerunInsideRepo pins the second-run UX: from inside the dotfiles
// repository, init needs no path flags — the enclosing repo and the stored
// answers supply everything, and the paths persist portably.
func TestInitRerunInsideRepo(t *testing.T) {
	home := initEnv(t)
	// A repo whose name and nesting do not match the <repos>/dotfiles default.
	repo := filepath.Join(home, "Repos", "me", "dotfiles.dotty")

	if err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=personal",
		"--addons=", "--agents=", "--dump-brews=false", "--on-conflict=backup",
		"--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Paths stored portably in the repo profile's answers.
	answers, err := scaffold.LoadAnswers(filepath.Join(repo, "profiles", "personal"))
	if err != nil {
		t.Fatal(err)
	}
	if answers.ReposDir != "~/Repos" || answers.Repo != filepath.Join("me", "dotfiles.dotty") {
		t.Fatalf("stored paths not portable: reposDir=%q repo=%q", answers.ReposDir, answers.Repo)
	}
	if _, err := os.Stat(filepath.Join(repo, ".dotty-version")); err != nil {
		t.Fatalf("repo marker missing: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(home, ".config", "dotty", "active-profile", "env.zsh"))
	if err != nil || !containsLine(string(env), `export REPOS_DIR="${HOME}/Repos"`) {
		t.Fatalf("env.zsh not home-relative: %v\n%s", err, env)
	}

	// Re-run from inside the repo with no path flags at all: the global
	// --profile picks the profile, the enclosing repo supplies the paths.
	initFlags = wizard.Flags{}
	t.Chdir(repo)
	t.Cleanup(func() { rootFlags.Profile = "" })
	if err := execDotty(t, "--profile=personal", "init",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run inside repo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Repos", "dotfiles")); err == nil {
		t.Fatal("re-run fell back to the default repo path")
	}

	// Re-run from anywhere: the linked profile's stored paths locate the repo.
	initFlags = wizard.Flags{}
	t.Chdir(home)
	if err := execDotty(t, "--profile=personal", "init",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run from home: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Repos", "dotfiles")); err == nil {
		t.Fatal("home re-run fell back to the default repo path")
	}
}

// TestInitRepoFlagAdoptsClone pins adoption-by-flag: on a machine with no
// dotty state, --repo naming an existing repository is interrogated for
// profiles and answers just like running from inside it.
func TestInitRepoFlagAdoptsClone(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	if err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=personal",
		"--addons=tmux", "--agents=", "--dump-brews=false", "--on-conflict=backup",
		"--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// A "new machine": the repository exists but no dotty config does, and
	// the working directory is outside the repository.
	if err := os.RemoveAll(filepath.Join(home, ".config", "dotty")); err != nil {
		t.Fatal(err)
	}
	initFlags = wizard.Flags{}
	t.Chdir(home)
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=personal",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("adopting init: %v", err)
	}

	answers, err := scaffold.LoadAnswers(filepath.Join(home, ".config", "dotty", "personal"))
	if err != nil {
		t.Fatalf("adopted profile not linked: %v", err)
	}
	if !slices.Equal(answers.AddOns, []string{"tmux"}) {
		t.Errorf("adopted add-ons = %v, want [tmux] from the repo's stored answers", answers.AddOns)
	}
}

// TestInitRerunExtendsProfile pins the extend-on-re-run flow: a re-run
// overlays new selections on the stored answers, renders what was added, and
// keeps every choice the re-run did not touch.
func TestInitRerunExtendsProfile(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	if err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=box",
		"--addons=tmux", "--agents=claude-code", "--marketplace", "--dump-brews=false",
		"--on-conflict=backup", "--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// A user-added Brewfile entry (dotty brewfile add, or by hand) must
	// survive the re-run — issue 109.
	brewPath := filepath.Join(repo, "profiles", "box", "Brewfile")
	brews, err := os.ReadFile(brewPath)
	if err != nil {
		t.Fatalf("read profile Brewfile: %v", err)
	}
	// The composed Brewfile carries no trailing newline; start a fresh line.
	brews = append(brews, "\nbrew \"user-added\"\n"...)
	if err := os.WriteFile(brewPath, brews, 0o644); err != nil {
		t.Fatal(err)
	}

	// Extend: a new add-on list; no path flags — the stored answers locate
	// the repository, and untouched answers (agents, marketplace) survive.
	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--profile-name=box", "--addons=tmux,lsd",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run: %v", err)
	}

	merged, err := os.ReadFile(brewPath)
	if err != nil {
		t.Fatalf("read profile Brewfile after re-run: %v", err)
	}
	if !containsLine(string(merged), `brew "user-added"`) {
		t.Errorf("user Brewfile entry destroyed by re-run:\n%s", merged)
	}
	if strings.Count(string(merged), `brew "tmux"`) != 1 {
		t.Errorf("tmux not exactly once after re-run:\n%s", merged)
	}

	answers, err := scaffold.LoadAnswers(filepath.Join(home, ".config", "dotty", "box"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(answers.AddOns, []string{"tmux", "lsd"}) {
		t.Errorf("add-ons after re-run = %v, want [tmux lsd]", answers.AddOns)
	}
	if !slices.Equal(answers.Agents, []string{"claude-code"}) || !answers.Marketplace {
		t.Errorf("untouched answers lost: agents=%v marketplace=%v", answers.Agents, answers.Marketplace)
	}
	if _, err := os.Stat(filepath.Join(repo, "home", ".config", "lsd")); err != nil {
		t.Errorf("extended add-on lsd not rendered: %v", err)
	}
}

func TestInitBacksUpConflicts(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	// A pre-existing real ghostty config conflicts with the link.
	existing := filepath.Join(home, ".config", "ghostty", "config")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=box",
		"--addons=", "--agents=", "--dump-brews=false", "--on-conflict=backup",
		"--yes", "--skip-font", "--skip-git")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// The user's file is recoverable from the backup mirror.
	backups := filepath.Join(home, ".local", "share", "dotty", "backups")
	sets, err := os.ReadDir(backups)
	if err != nil || len(sets) != 1 {
		t.Fatalf("backup sets = %v, %v", sets, err)
	}
	mirror := filepath.Join(backups, sets[0].Name(), existing)
	if got, err := os.ReadFile(mirror); err != nil || string(got) != "mine" {
		t.Fatalf("backup mirror = %q, %v", got, err)
	}

	// And dotfiles restore puts it back over the link.
	if err := execDotty(t, "dotfiles", "restore", "--timestamp="+sets[0].Name()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != "mine" {
		t.Fatalf("restored file = %q, %v", got, err)
	}
	if info, _ := os.Lstat(existing); info != nil && info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("restore left a symlink in place")
	}
}

func TestDotfilesStatusAndLink(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")
	t.Setenv("REPOS_DIR", filepath.Join(home, "Repos"))

	if err := execDotty(t, "init", "--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"),
		"--profile-name=box", "--addons=", "--agents=", "--dump-brews=false",
		"--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Break a link, then status should flag it and link should heal it.
	link := filepath.Join(home, ".config", "ghostty")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "nowhere"), link); err != nil {
		t.Fatal(err)
	}

	if err := execDotty(t, "dotfiles", "status"); err != nil {
		t.Fatalf("status: %v", err)
	}
	if err := execDotty(t, "dotfiles", "link", "--on-conflict=fail"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if got, err := os.Readlink(link); err != nil || got != filepath.Join(repo, "home", ".config", "ghostty") {
		t.Fatalf("healed link = %q, %v", got, err)
	}
}

func TestInitSecurityKeys(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=keys",
		"--addons=", "--agents=", "--dump-brews=false", "--security-keys",
		"--git-name=Ada Lovelace", "--git-email=ada@example.com", "--allowed-serials=111,222",
		"--on-conflict=backup", "--yes", "--skip-font", "--skip-git", "--skip-keys")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { initFlags.AllowedSerials = nil })

	// The allowlist is part of the profile's answers, traveling with the repo.
	keysAnswers, err := scaffold.LoadAnswers(filepath.Join(repo, "profiles", "keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(keysAnswers.AllowedSerials, "111") || slices.Contains(keysAnswers.AllowedSerials, "999") {
		t.Fatalf("allowlist wrong: %v", keysAnswers.AllowedSerials)
	}

	assertKeyWiring(t, home, repo)

	// Never overwritten: a hand-edited private config survives a re-run.
	private := filepath.Join(home, ".config", "private", "git", "config")
	if err := os.WriteFile(private, []byte("[user]\n\tname = Edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "init", "--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"),
		"--profile-name=keys", "--yes", "--skip-font", "--skip-git", "--skip-keys", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if got, _ := os.ReadFile(private); !containsLine(string(got), "\tname = Edited") {
		t.Errorf("re-run overwrote the private config:\n%s", got)
	}

	// The Brewfile picked up the security-key packages.
	brew, err := os.ReadFile(filepath.Join(repo, "profiles", "keys", "Brewfile"))
	if err != nil || !containsLine(string(brew), `brew "ykman"`) {
		t.Errorf("Brewfile missing ykman: %v", err)
	}
}

// assertKeyWiring checks the security-key artifacts an init with
// --security-keys produces: the private ~/.ssh, the profile's signing
// include, the askpass applet symlink, and the private identity file.
func assertKeyWiring(t *testing.T, home, repo string) {
	t.Helper()
	info, err := os.Stat(filepath.Join(home, ".ssh"))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("~/.ssh: %v, perm %o (want 700)", err, info.Mode().Perm())
	}
	sshConfig, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil || !containsLine(string(sshConfig), `Match host * exec "dotty signing-key link >/dev/null"`) {
		t.Errorf("~/.ssh/config: %v\n%s", err, sshConfig)
	}

	gitcfg, err := os.ReadFile(filepath.Join(home, ".config", "dotty", "active-profile", "git.gitconfig"))
	if err != nil || !containsLine(string(gitcfg), "\tformat = ssh") {
		t.Errorf("profile git.gitconfig missing gpg block: %v\n%s", err, gitcfg)
	}

	askpass := filepath.Join(home, ".local", "share", "dotty", "dotty-ssh-askpass")
	self, _ := os.Executable()
	if link, err := os.Readlink(askpass); err != nil || link != self {
		t.Errorf("askpass symlink = %q, %v (want %s)", link, err, self)
	}

	private := filepath.Join(home, ".config", "private", "git", "config")
	data, err := os.ReadFile(private)
	if err != nil {
		t.Fatalf("private git config: %v", err)
	}
	for _, want := range []string{"\tname = Ada Lovelace", "\temail = ada@example.com", "\tgpgSign = true"} {
		if !containsLine(string(data), want) {
			t.Errorf("private config missing %q:\n%s", want, data)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "home", ".config", "private")); err == nil {
		t.Error("private identity leaked into the repository")
	}
}

// TestInitUsesExistingProfileKeys pins the adopted-machine flow: stubs on
// disk are offered and (non-interactively) used as-is — with or without the
// profile knowing them through an allowlist — instead of asking to enroll.
func TestInitUsesExistingProfileKeys(t *testing.T) {
	for _, allowed := range []string{"", "111"} {
		t.Run("allowlist="+allowed, func(t *testing.T) {
			testInitUsesExistingKeys(t, allowed)
		})
	}
}

func testInitUsesExistingKeys(t *testing.T, allowed string) {
	t.Helper()
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	// Minimal git identity so trusting the keys has an email to pair with.
	gitcfg := filepath.Join(home, "gitconfig")
	gitcfgBody := "[user]\n\temail = t@x\n[gpg \"ssh\"]\n\tallowedSignersFile = ~/allowed\n"
	if err := os.WriteFile(gitcfg, []byte(gitcfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitcfg)

	// A stub on disk for serial 111.
	dataDir := filepath.Join(home, ".local", "share", "dotty")
	stub := signingkey.StubPath(dataDir, "111", "ed25519", "u")
	if err := os.MkdirAll(filepath.Dir(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{stub: "stub", stub + ".pub": "ssh-ed25519 AAAA u"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// No --skip-keys, so the non-interactive run auto-uses the stub.
	initFlags = wizard.Flags{}
	args := []string{"init",
		"--repo=" + repo, "--repos-dir=" + filepath.Join(home, "Repos"), "--profile-name=adopted",
		"--addons=", "--agents=", "--dump-brews=false", "--security-keys",
		"--git-name=T", "--git-email=t@x", "--on-conflict=backup", "--yes", "--skip-font", "--skip-git"}
	if allowed != "" {
		args = append(args, "--allowed-serials="+allowed)
	}
	if err := execDotty(t, args...); err != nil {
		t.Fatalf("init: %v", err)
	}

	link, err := os.Readlink(signingkey.DefaultLinkPath(home))
	if err != nil || link != stub {
		t.Fatalf("ssh identity link = %q, %v (want %s)", link, err, stub)
	}
	if got, err := os.ReadFile(filepath.Join(home, "allowed")); err != nil || !strings.Contains(string(got), "t@x") {
		t.Fatalf("allowed_signers not written: %v\n%s", err, got)
	}
}

func TestInitWithoutSecurityKeys(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	err := execDotty(t, "init",
		"--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"), "--profile-name=nokeys",
		"--addons=", "--agents=", "--dump-brews=false", "--security-keys=false",
		"--git-name=Ada", "--git-email=ada@example.com",
		"--on-conflict=backup", "--yes", "--skip-font", "--skip-git", "--skip-keys")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// No gpg config anywhere, unsigned commits, no ssh config rendered.
	gitcfg, err := os.ReadFile(filepath.Join(home, ".config", "dotty", "active-profile", "git.gitconfig"))
	if err != nil {
		t.Fatalf("profile git.gitconfig: %v", err)
	}
	if containsLine(string(gitcfg), "\tformat = ssh") {
		t.Errorf("gpg block rendered without security keys:\n%s", gitcfg)
	}
	private, _ := os.ReadFile(filepath.Join(home, ".config", "private", "git", "config"))
	if !containsLine(string(private), "\tgpgSign = false") {
		t.Errorf("private config should disable signing:\n%s", private)
	}
	if _, err := os.Stat(filepath.Join(repo, "home", ".ssh")); err == nil {
		t.Error(".ssh rendered without security keys")
	}
}

// TestInitPrunesRelocatedRenders pins the re-run cleanup on a repository
// already in the current layout: a profile render orphaned by a template
// relocation (~/.claude/settings.json moved under ~/.config/claude) is
// pruned, and the live symlink an older dotty left pointing at it — dangling
// once the render is gone — is removed with it.
func TestInitPrunesRelocatedRenders(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box", "--agents=claude-code",
		"--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// A render at the pre-XDG destination, and the live link an older dotty
	// left pointing at it through the active-profile chain.
	profileDir := filepath.Join(repo, "profiles", "box")
	orphan := filepath.Join(profileDir, "home", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(site), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".config", "dotty", "active-profile", "home", ".claude", "settings.json")
	if err := os.Symlink(target, site); err != nil {
		t.Fatal(err)
	}

	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run: %v", err)
	}

	if _, err := os.Lstat(orphan); err == nil {
		t.Error("orphaned render survived the re-run")
	}
	if _, err := os.Lstat(filepath.Join(profileDir, "home", ".claude")); err == nil {
		t.Error("emptied .claude directory survived the re-run")
	}
	if _, err := os.Lstat(site); err == nil {
		t.Error("dangling live link survived the re-run")
	}
	if _, err := os.Stat(filepath.Join(profileDir, "home", ".config", "claude", "settings.json")); err != nil {
		t.Errorf("current render missing after the prune: %v", err)
	}
}

// TestInitMigratesCodexMachineState pins the codex config migration: a live
// ~/.config/codex/config.toml symlinked through active-profile (the pre-layer
// layout) becomes a real machine-local file carrying the profile render's
// content — Codex's project trust and desktop state — with a backup set, while
// the orphaned render is pruned and dotty.config.toml takes over as the
// linked profile layer.
func TestInitMigratesCodexMachineState(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")

	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box", "--agents=codex",
		"--yes", "--skip-font", "--skip-git"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Seed the pre-layer world: a config.toml render in the profile holding
	// machine state Codex wrote through the link, and the live symlink an
	// older dotty left pointing at it.
	const state = "[projects.\"/x\"]\ntrust_level = \"trusted\"\n"
	profileDir := filepath.Join(repo, "profiles", "box")
	legacy := filepath.Join(profileDir, "home", ".config", "codex", "config.toml")
	if err := os.WriteFile(legacy, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(home, ".config", "codex", "config.toml")
	target := filepath.Join(home, ".config", "dotty", "active-profile", "home", ".config", "codex", "config.toml")
	if err := os.Symlink(target, site); err != nil {
		t.Fatal(err)
	}

	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("re-run: %v", err)
	}

	info, err := os.Lstat(site)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("live config.toml is not a regular file: %v, %v", info, err)
	}
	got, err := os.ReadFile(site)
	if err != nil || string(got) != state {
		t.Fatalf("machine state lost in migration: %q, %v", got, err)
	}
	if _, err := os.Lstat(legacy); err == nil {
		t.Error("orphaned config.toml render survived the re-run")
	}
	if _, err := os.Stat(filepath.Join(profileDir, "home", ".config", "codex", "dotty.config.toml")); err != nil {
		t.Errorf("profile layer missing after re-run: %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(home, ".local", "share", "dotty", "backups", "*",
		strings.TrimPrefix(site, string(filepath.Separator))))
	if err != nil || len(backups) != 1 {
		t.Fatalf("want one backup mirror of %s, got %v, %v", site, backups, err)
	}

	// Third run: the real file is Codex's now, and init must not disturb it.
	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=fail"); err != nil {
		t.Fatalf("third run: %v", err)
	}
	if got, err := os.ReadFile(site); err != nil || string(got) != state {
		t.Fatalf("machine-local file changed on re-run: %q, %v", got, err)
	}
}

// containsLine reports whether text has line as one exact line.
func containsLine(text, line string) bool {
	for l := range strings.Lines(text) {
		if strings.TrimSuffix(l, "\n") == line {
			return true
		}
	}
	return false
}

// seedLegacyLayout lays down a repository from before the layout change — the
// old stow/ tree name, the old profile location with split
// dotty.json/profile.json documents, and a stale render missing current
// hardening — plus machine-local legacy state: a real profile directory where
// the link into the repository belongs, holding its own stale render.
func seedLegacyLayout(t *testing.T, home, repo string) {
	t.Helper()
	const (
		legacyAnswers = `{"profile":"box","reposDir":"~/Repos","repo":"dotfiles","agents":["claude-code"],"harden":true}`
		legacyMeta    = `{"name":"box","description":"legacy box","created_at":"2025-01-02T03:04:05Z"}`
	)
	legacyProfile := filepath.Join(repo, "stow", ".config", "dotty", "box")
	localProfile := filepath.Join(home, ".config", "dotty", "box")
	files := map[string]string{
		filepath.Join(repo, "stow", ".config", "ghostty", "config"):                          "old ghostty",
		filepath.Join(legacyProfile, "dotty.json"):                                           legacyAnswers,
		filepath.Join(legacyProfile, "profile.json"):                                         legacyMeta,
		filepath.Join(legacyProfile, "Brewfile"):                                             `brew "tmux"`,
		filepath.Join(legacyProfile, "render", "env.zsh"):                                    "# stale env",
		filepath.Join(legacyProfile, "render", "stow", ".config", "claude", "settings.json"): `{"stale":true}`,
		filepath.Join(legacyProfile, "render", "stow", ".claude", "settings.json"):           `{"stale":true}`,
		filepath.Join(localProfile, "render", "stow", ".config", "claude", "settings.json"):  `{"stale":true}`,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestInitMigratesLegacyLayout pins the upgrade path: a repository from
// before the layout change — a stow/ tree, profiles inside it under
// .config/dotty with a render/ ceremony and split dotty.json/profile.json —
// is restructured by a re-run, its stale renders regenerated, and the live
// links healed to the new paths.
func TestInitMigratesLegacyLayout(t *testing.T) {
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")
	localProfile := filepath.Join(home, ".config", "dotty", "box")
	seedLegacyLayout(t, home, repo)

	initFlags = wizard.Flags{}
	if err := execDotty(t, "init", "--repo="+repo, "--profile-name=box",
		"--yes", "--skip-font", "--skip-git", "--on-conflict=backup"); err != nil {
		t.Fatalf("migrating init: %v", err)
	}

	// The repository is restructured: home/ tree, top-level profile, one
	// merged document, no render ceremony.
	profileDir := filepath.Join(repo, "profiles", "box")
	for _, gone := range []string{
		filepath.Join(repo, "stow"),
		filepath.Join(profileDir, "render"),
		filepath.Join(profileDir, "dotty.json"),
		// The pre-XDG render the migration carried across is pruned — the
		// current plan renders claude settings under .config/claude only.
		filepath.Join(profileDir, "home", ".claude"),
	} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s survived the migration", gone)
		}
	}
	if got, err := os.ReadFile(filepath.Join(repo, "home", ".config", "ghostty", "config")); err != nil {
		t.Errorf("home tree missing migrated ghostty config: %v", err)
	} else if string(got) == "" {
		t.Error("ghostty config empty after migration")
	}
	answers, err := scaffold.LoadAnswers(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	if !answers.Harden || !slices.Equal(answers.Agents, []string{"claude-code"}) {
		t.Errorf("merged answers lost selections: %+v", answers)
	}
	// The legacy profile's Brewfile entries survive the migration re-render.
	if brews, err := os.ReadFile(filepath.Join(profileDir, "Brewfile")); err != nil {
		t.Errorf("migrated Brewfile missing: %v", err)
	} else if !containsLine(string(brews), `brew "tmux"`) {
		t.Errorf("migrated Brewfile lost the legacy tmux entry:\n%s", brews)
	}
	if answers.Description != "legacy box" || answers.CreatedAt.IsZero() {
		t.Errorf("metadata lost in merge: description=%q created=%v", answers.Description, answers.CreatedAt)
	}

	// The stale render was regenerated with the current hardening, and the
	// live chain resolves to it: local real dir replaced by a repo link.
	rendered, err := os.ReadFile(filepath.Join(profileDir, "home", ".config", "claude", "settings.json"))
	if err != nil || strings.Contains(string(rendered), "stale") {
		t.Fatalf("profile settings.json not regenerated: %v\n%s", err, rendered)
	}
	if link, err := os.Readlink(localProfile); err != nil || link != profileDir {
		t.Errorf("~/.config/dotty/box = %q, %v (want link to %s)", link, err, profileDir)
	}
	live, err := os.ReadFile(filepath.Join(home, ".config", "claude", "settings.json"))
	if err != nil || strings.Contains(string(live), "stale") {
		t.Errorf("live settings.json still stale through active-profile: %v", err)
	}
}
