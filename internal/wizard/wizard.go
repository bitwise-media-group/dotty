// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package wizard

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/macos"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/securitykey"
	"github.com/bitwise-media-group/dotty/internal/tmux"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// Flags holds the options for `dotty init`. Every wizard question has a
// mirroring flag so the command also runs unattended; the trailing fields
// (OnConflict onward) steer the steps after the interview.
type Flags struct {
	Repo           string
	ReposDir       string
	ProfileName    string
	AddOns         []string
	Agents         []string
	DumpBrews      bool
	Marketplace    bool
	Harden         bool
	SecurityKeys   bool
	GitName        string
	GitEmail       string
	AllowedSerials []string
	Worktrees      string
	MacOSDefaults  []string
	Wallpaper      string
	PIV            bool
	OnConflict     string
	// Yes skips the confirmation summary and, on a re-run, every question the
	// stored profile already answers — only the profile pick and questions the
	// stored document predates are asked.
	Yes      bool
	SkipFont bool
	SkipGit  bool
	SkipKeys bool
}

// Collect resolves the wizard answers: the profile first (so a machine with
// existing profiles picks instead of typing), then the full interview. A
// previous run's persisted answers seed every question's default, so a
// re-run walks the same interview to extend the profile — pressing enter
// throughout keeps it as it is — while non-interactive re-runs take the
// stored answers as-is. Flags override in both cases, and --yes silences
// every question the stored profile already answers, so a --yes re-run asks
// only the profile pick plus whatever the stored document predates. Backing
// out of any prompt surfaces as tui.ErrAborted. It returns the answers, the
// resolved repository path, and whether an existing profile's answers seeded
// the run.
func Collect(ios cli.IOStreams, flags Flags, home string) (scaffold.Answers, string, bool, error) {
	var none scaffold.Answers

	// The repository whose profiles seed the interview: an explicit --repo
	// naming an existing dotty repository, else the one enclosing the working
	// directory — so a new machine adopts a fresh clone from inside it or by
	// pointing at it.
	enclosing := scaffold.EnclosingRepo()
	if flags.Repo != "" {
		if dir, err := cli.ExpandHome(flags.Repo); err == nil && scaffold.IsRepo(dir) {
			enclosing = dir
		}
	}

	profileName, err := resolveProfileName(ios, flags, enclosing)
	if err != nil {
		return none, "", false, err
	}

	// The enclosing repository wins over the linked profile so running
	// inside a fresh clone adopts that clone.
	prev, known, prevRepo, rerun, err := previousAnswers(profileName, enclosing, home)
	if err != nil {
		return none, "", false, err
	}

	reposDir, repo, err := resolvePaths(ios, flags, prev, known, prevRepo, enclosing, home, rerun)
	if err != nil {
		return none, "", false, err
	}

	var answers scaffold.Answers
	if rerun {
		answers = mergeAnswers(prev, flags, reposDir, repo, home)
	} else {
		answers = scaffold.Answers{ProfileName: profileName,
			AddOns: flags.AddOns, Agents: flags.Agents, DumpBrews: flags.DumpBrews,
			Marketplace: flags.Marketplace, Harden: flags.Harden, SecurityKeys: flags.SecurityKeys,
			MacOSDefaults: flags.MacOSDefaults, Wallpaper: flags.Wallpaper, PIV: flags.PIV,
			AllowedSerials: flags.AllowedSerials, Worktrees: flags.Worktrees}
		setAnswerPaths(&answers, reposDir, repo, home)
	}
	if answers.Worktrees == "" {
		answers.Worktrees = scaffold.DefaultWorktrees
	}
	for _, serial := range answers.AllowedSerials {
		if !securitykey.IsSerial(serial) {
			return none, "", false, fmt.Errorf("--allowed-serials: %q is not a serial number", serial)
		}
	}

	if err := collectSelections(ios, flags, known, &answers); err != nil {
		return none, "", false, err
	}
	if err := collectMacOSAnswers(ios, flags, known, home, &answers); err != nil {
		return none, "", false, err
	}
	return answers, repo, rerun, nil
}

// reuses reports whether --yes silences a question: answered is the matching
// AnswerKeys field, true when the stored profile carries that answer to
// reuse. Without --yes, or for a question the stored document predates,
// the interview still asks.
func (f Flags) reuses(answered bool) bool {
	return f.Yes && answered
}

// ConfirmSummary recaps what init will do and asks once; false means the
// user backed out.
func ConfirmSummary(ios cli.IOStreams, a scaffold.Answers, repo string, rerun bool) (bool, error) {
	verb := "create"
	if rerun {
		verb = "re-render"
	}
	tui.Infof(ios, "About to %s %s: profile %q, add-ons [%s], agents [%s], then link into $HOME.",
		verb, repo, a.ProfileName, strings.Join(a.AddOns, " "), strings.Join(a.Agents, " "))
	ok, err := tui.Confirm(ios, "Proceed?", "")
	if errors.Is(err, tui.ErrNotInteractive) {
		return false, errors.New("cannot confirm without a terminal; pass --yes")
	}
	if errors.Is(err, tui.ErrAborted) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ok, nil
}

// resolvePaths asks the two path questions — the repositories directory and
// the dotfiles repository — defaulting to the stored locations on a re-run
// and to the enclosing repository when run from inside one. --yes takes a
// stored location without asking.
func resolvePaths(ios cli.IOStreams, flags Flags, prev scaffold.Answers, known scaffold.AnswerKeys,
	prevRepo, enclosing, home string, rerun bool) (string, string, error) {
	reposDir := flags.ReposDir
	if reposDir == "" {
		def := filepath.Join(home, "Repos")
		if rerun && prev.ReposDir != "" {
			def = scaffold.ExpandTilde(prev.ReposDir, home)
		}
		if flags.reuses(known.ReposDir) {
			reposDir = def
		} else {
			var err error
			if reposDir, err = askPath(ios, "Where do you keep repositories?", def, homeDirSuggestions(home)); err != nil {
				return "", "", err
			}
		}
	}
	reposDir, err := cli.ExpandHome(reposDir)
	if err != nil {
		return "", "", err
	}

	repo := flags.Repo
	if repo == "" {
		def := filepath.Join(reposDir, "dotfiles")
		if enclosing != "" {
			def = enclosing
		}
		if rerun {
			def = prevRepo
		}
		if flags.reuses(known.Repo) {
			repo = def
		} else if repo, err = askPath(ios, "Where should the dotfiles repository live?", def,
			tmux.FindRepos(reposDir, 4)); err != nil {
			return "", "", err
		}
	}
	if repo, err = cli.ExpandHome(repo); err != nil {
		return "", "", err
	}
	return reposDir, repo, nil
}

// resolveProfileName picks the profile init targets: the flag (--profile-name
// or the global --profile), a picklist when this machine or the enclosing
// repository already has profiles, or a typed name for the first one.
func resolveProfileName(ios cli.IOStreams, flags Flags, enclosing string) (string, error) {
	name := flags.ProfileName
	if name == "" && ios.IsInteractive() {
		existing := knownProfiles(enclosing)
		if len(existing) > 0 {
			options := make([]tui.Option, 0, len(existing)+1)
			for _, p := range existing {
				options = append(options, tui.Option{Label: p, Value: p})
			}
			options = append(options, tui.Option{Label: "create a new profile", Value: ""})
			picked, err := tui.FuzzySelect(ios, "Which profile is this machine?", options)
			if err != nil {
				return "", err
			}
			name = picked
		}
	}
	if name == "" {
		var err error
		if name, err = askDefault(ios, "Profile name?", defaultProfileName()); err != nil {
			return "", err
		}
	}
	return name, profile.ValidateName(name)
}

// knownProfiles lists the profiles this machine can see: the linked (or
// local) ones under the config dir, plus any in the enclosing repository.
func knownProfiles(enclosing string) []string {
	seen := map[string]bool{}
	if configDir, err := cli.ConfigDir(); err == nil {
		if profiles, err := profile.List(configDir); err == nil {
			for _, p := range profiles {
				seen[p.Name] = true
			}
		}
	}
	if enclosing != "" {
		for _, name := range scaffold.ListRepoProfiles(enclosing) {
			seen[name] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// previousAnswers finds a profile's persisted answers: in the enclosing
// repository first (running inside a fresh clone adopts it), then through
// the linked profile — whose stored ReposDir+Repo locate the repository. The
// AnswerKeys alongside them say which questions that document answers.
func previousAnswers(profileName, enclosing, home string) (
	scaffold.Answers, scaffold.AnswerKeys, string, bool, error) {
	var none scaffold.Answers
	var noKeys scaffold.AnswerKeys
	if enclosing != "" {
		prev, known, err := scaffold.LoadRepoAnswers(enclosing, profileName)
		if err == nil {
			return prev, known, enclosing, true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return none, noKeys, "", false, err
		}
	}
	configDir, err := cli.ConfigDir()
	if err != nil {
		return none, noKeys, "", false, err
	}
	prev, known, err := scaffold.LoadAnswersWithKeys(profile.Dir(configDir, profileName))
	if errors.Is(err, fs.ErrNotExist) {
		return none, noKeys, "", false, nil
	}
	if err != nil {
		return none, noKeys, "", false, err
	}
	reposDir := scaffold.ExpandTilde(prev.ReposDir, home)
	repo := scaffold.ExpandTilde(prev.Repo, home)
	if repo == "" {
		repo = filepath.Join(reposDir, "dotfiles")
	} else if !filepath.IsAbs(repo) {
		repo = filepath.Join(reposDir, repo)
	}
	return prev, known, repo, true, nil
}

// setAnswerPaths stores the paths portably: ReposDir home-relative, Repo
// relative to ReposDir when inside it.
func setAnswerPaths(a *scaffold.Answers, reposDir, repo, home string) {
	a.ReposDir = scaffold.TildePath(reposDir, home)
	if rel, err := filepath.Rel(reposDir, repo); err == nil && !strings.HasPrefix(rel, "..") {
		a.Repo = rel
	} else {
		a.Repo = scaffold.TildePath(repo, home)
	}
}

// homeDirSuggestions offers the non-hidden directories under home as
// completions for the repositories-directory question.
func homeDirSuggestions(home string) []string {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(home, e.Name()))
		}
	}
	return dirs
}

// askPath prompts for a path with tab-completable suggestions;
// non-interactive runs and empty answers take the default.
func askPath(ios cli.IOStreams, title, def string, suggestions []string) (string, error) {
	got, err := tui.InputSuggest(ios, title, def, suggestions, nil)
	if errors.Is(err, tui.ErrNotInteractive) {
		return def, nil
	}
	if err != nil {
		return "", err
	}
	if got == "" {
		return def, nil
	}
	return got, nil
}

// collectSelections asks the what-goes-on-this-machine-class questions:
// Brewfile seeding, add-ons, agents, and — once agents are chosen — the
// marketplace, hardening, and security keys. Flags silence their question,
// as does --yes when the stored profile already answers it; everything else
// is asked with the seeded answer (a previous run's, or the zero value) as
// the default, so a re-run revisits every choice.
func collectSelections(ios cli.IOStreams, flags Flags, known scaffold.AnswerKeys, answers *scaffold.Answers) error {
	var err error
	if !flags.DumpBrews && ios.IsInteractive() && !flags.reuses(known.DumpBrews) {
		if answers.DumpBrews, err = tui.ConfirmDefault(ios, "Seed the Brewfile from what is installed now?",
			"brew bundle dump, merged with the template's packages", answers.DumpBrews); err != nil {
			return err
		}
	}

	if flags.AddOns == nil && ios.IsInteractive() && !flags.reuses(known.AddOns) {
		if answers.AddOns, err = askMulti(ios, "Which add-ons do you want?", addOnOptions, answers.AddOns); err != nil {
			return err
		}
	} else if answers.AddOns == nil {
		answers.AddOns = []string{}
	}
	if flags.Agents == nil && ios.IsInteractive() && !flags.reuses(known.Agents) {
		if answers.Agents, err = askMulti(ios, "Which coding agents do you use?", agentOptions, answers.Agents); err != nil {
			return err
		}
	} else if answers.Agents == nil {
		answers.Agents = []string{}
	}
	return collectFeatureConfirms(ios, flags, known, answers)
}

// collectFeatureConfirms asks the yes/no feature questions that follow the
// picklists: the marketplace and hardening (which only mean something once
// at least one agent is selected) and security keys.
func collectFeatureConfirms(
	ios cli.IOStreams, flags Flags, known scaffold.AnswerKeys, answers *scaffold.Answers,
) error {
	if !ios.IsInteractive() {
		return nil
	}
	var err error
	// The marketplace only means something once at least one agent will use
	// it; choosing it wires the bitwise skills into every selected agent that
	// supports marketplaces.
	if len(answers.Agents) > 0 && !flags.Marketplace && !flags.reuses(known.Marketplace) {
		if answers.Marketplace, err = tui.ConfirmDefault(ios, "Add the bitwise skills marketplace?",
			"github.com/bitwise-media-group/skills, enabled for all selected agents", answers.Marketplace); err != nil {
			return err
		}
	}

	if !flags.SecurityKeys && !flags.reuses(known.SecurityKeys) {
		if answers.SecurityKeys, err = tui.ConfirmDefault(ios, "Do you use security keys (YubiKey)?",
			"wires SSH and git signing through the hardware key, and adds ykman + pinentry-mac to the Brewfile",
			answers.SecurityKeys); err != nil {
			return err
		}
	}

	// Hardening confines each selected agent the way Claude Code's sandbox
	// confines it: sandboxed writes, credential-path read denies, and
	// prompt-unless-allowlisted permissions, mirrored into every agent's
	// native config.
	if len(answers.Agents) > 0 && !flags.Harden && !flags.reuses(known.Harden) {
		if answers.Harden, err = tui.ConfirmDefault(ios, "Harden the coding agents?",
			"sandbox + deny credential reads + ask-first permissions, in each agent's native config",
			answers.Harden); err != nil {
			return err
		}
	}
	return nil
}

// collectMacOSAnswers asks the darwin-only questions: which defaults groups
// to apply (everything preselected on a first run, the stored picks on a
// re-run), a wallpaper when the conventional directory has any, and PIV
// enforcement (off unless chosen — it can lock a machine without an
// enrolled card out). Like the interview proper, --yes reuses whatever the
// stored profile answers.
func collectMacOSAnswers(ios cli.IOStreams, flags Flags, known scaffold.AnswerKeys,
	home string, answers *scaffold.Answers) error {
	if runtime.GOOS != "darwin" || !ios.IsInteractive() {
		return nil
	}

	if flags.MacOSDefaults == nil && !flags.reuses(known.MacOSDefaults) {
		options := make([]tui.Option, len(macos.Groups))
		for i, g := range macos.Groups {
			options[i] = tui.Option{Label: g.Label, Value: g.ID,
				Selected: answers.MacOSDefaults == nil || slices.Contains(answers.MacOSDefaults, g.ID)}
		}
		picked, err := tui.MultiSelect(ios, "Which macOS defaults should apply?", options)
		if err != nil {
			return err
		}
		answers.MacOSDefaults = picked
	}

	if flags.Wallpaper == "" && !flags.reuses(known.Wallpaper) {
		if images := macos.Wallpapers(filepath.Join(home, ".local", "share", "wallpapers")); len(images) > 0 {
			options := []tui.Option{{Label: "keep the current wallpaper", Value: ""}}
			for _, img := range images {
				options = append(options, tui.Option{Label: img, Value: img})
			}
			picked, err := tui.FuzzySelect(ios, "Choose a wallpaper?", options)
			if err != nil && !errors.Is(err, tui.ErrAborted) {
				return err
			}
			// Keeping the current wallpaper keeps the stored answer too, so a
			// re-run does not forget the machine class's wallpaper.
			if picked != "" {
				answers.Wallpaper = picked
			}
		}
	}

	if !flags.PIV && !flags.reuses(known.PIV) {
		ok, err := tui.ConfirmDefault(ios, "Require smart-card (PIV) login?",
			"system-wide, via sudo — do not enable without an enrolled card, or you lock yourself out",
			answers.PIV)
		if err != nil {
			return err
		}
		answers.PIV = ok
	}
	return nil
}

var addOnOptions = []tui.Option{
	{Label: "neovim (with lazygit)", Value: "nvim"},
	{Label: "btop", Value: "btop"},
	{Label: "k9s", Value: "k9s"},
	{Label: "lazygit", Value: "lazygit"},
	{Label: "lsd", Value: "lsd"},
	{Label: "tmux", Value: "tmux"},
	{Label: "yazi", Value: "yazi"},
}

var agentOptions = []tui.Option{
	{Label: "claude code", Value: "claude-code"},
	{Label: "codex", Value: "codex"},
	{Label: "opencode", Value: "opencode"},
	{Label: "antigravity", Value: "antigravity"},
	{Label: "grok", Value: "grok"},
}

// mergeAnswers overlays explicitly-passed flags onto a previous run's
// persisted answers, restoring portable path storage for whatever changed.
func mergeAnswers(prev scaffold.Answers, flags Flags, reposDir, repo, home string) scaffold.Answers {
	setAnswerPaths(&prev, reposDir, repo, home)
	if flags.DumpBrews {
		prev.DumpBrews = true
	}
	if flags.AddOns != nil {
		prev.AddOns = flags.AddOns
	}
	if flags.Agents != nil {
		prev.Agents = flags.Agents
	}
	if flags.Marketplace {
		prev.Marketplace = true
	}
	if flags.SecurityKeys {
		prev.SecurityKeys = true
	}
	if flags.Harden {
		prev.Harden = true
	}
	if flags.MacOSDefaults != nil {
		prev.MacOSDefaults = flags.MacOSDefaults
	}
	if flags.Wallpaper != "" {
		prev.Wallpaper = flags.Wallpaper
	}
	if flags.PIV {
		prev.PIV = true
	}
	if flags.Worktrees != "" {
		prev.Worktrees = flags.Worktrees
	}
	if flags.AllowedSerials != nil {
		prev.AllowedSerials = flags.AllowedSerials
	}
	return prev
}

// askDefault prompts for a value with def as the placeholder;
// non-interactive runs and empty answers take the default.
func askDefault(ios cli.IOStreams, title, def string) (string, error) {
	got, err := tui.Input(ios, title, def, nil)
	if errors.Is(err, tui.ErrNotInteractive) {
		return def, nil
	}
	if err != nil {
		return "", err
	}
	if got == "" {
		return def, nil
	}
	return got, nil
}

// askMulti prompts a multi-select with the selected values preticked;
// non-interactive runs select nothing.
func askMulti(ios cli.IOStreams, title string, options []tui.Option, selected []string) ([]string, error) {
	opts := slices.Clone(options)
	for i := range opts {
		opts[i].Selected = slices.Contains(selected, opts[i].Value)
	}
	picked, err := tui.MultiSelect(ios, title, opts)
	if errors.Is(err, tui.ErrNotInteractive) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return picked, nil
}

// defaultProfileName is the machine name, lowercased, without the mDNS
// suffix.
func defaultProfileName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "default"
	}
	return strings.ToLower(strings.TrimSuffix(host, ".local"))
}
