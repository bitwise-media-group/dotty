// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/version"
)

// AnswersFile is the profile document: the wizard's persisted answers plus
// the profile's metadata (description, creation time), committed inside the
// profile's directory so it travels between machines.
const AnswersFile = "profile.json"

// legacyAnswersFile is where answers lived before they merged into
// profile.json; LoadAnswers still reads it so re-running init migrates old
// repositories.
const legacyAnswersFile = "dotty.json"

// Answers is everything the init wizard collects. Later phases append fields
// (git identity, security keys, hardening); zero values keep a Phase-1 repo
// rendering identically when they arrive.
//
// Answer fields serialize even when falsy — an answered "no" must survive the
// round-trip so a --yes re-run can tell it apart from a question the stored
// document never answered. Slices carry that distinction in their nilness:
// omitzero drops a nil (never-asked) slice but keeps an answered-empty [].
type Answers struct {
	ProfileName string `json:"profile"`
	// Description and CreatedAt are the profile's metadata, kept in the same
	// document (they share profile.json with the answers; the profile
	// package's Profile reads the same keys).
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitzero"`
	// ReposDir and Repo are stored portably so answers travel across
	// machines with different homes: ReposDir folds a home prefix to ~, and
	// Repo is relative to ReposDir (absolute only when outside it).
	ReposDir string   `json:"reposDir"`
	Repo     string   `json:"repo"`
	AddOns   []string `json:"addons,omitzero"`
	Agents   []string `json:"agents,omitzero"`
	// ImportPackages seeds the profile's packages from the Homebrew formulae
	// installed on the machine. It was dumpBrews before packages moved to
	// mise; readAnswers still honours the old key.
	ImportPackages bool `json:"importPackages"`

	SecurityKeys bool `json:"securityKeys"`
	Harden       bool `json:"harden"`
	Marketplace  bool `json:"marketplace"`
	// HushLogin renders ~/.hushlogin, silencing login(1)'s banner. It
	// defaults on: a document that predates the question reads as true.
	HushLogin bool `json:"hushLogin"`

	MacOSDefaults []string `json:"macosDefaults,omitzero"`
	// DockApps picks which of the optional apps (ghostty, claude-code,
	// codex — the agents' desktop apps go by the agent's id) the dock
	// defaults group pins. nil, never asked, pins every eligible one.
	DockApps []string `json:"dockApps,omitzero"`
	PIV      bool     `json:"piv"`

	// AllowedSerials restricts the profile's machines to these security-key
	// serials; empty means unrestricted. Serials identify hardware (they are
	// etched on the key), and the restriction is a property of the machine
	// class, so it travels with the profile and swaps on activation.
	AllowedSerials []string `json:"allowedSerials,omitzero"`

	// Worktrees is where agent worktrees live: a directory name relative to
	// each repository (the default, .worktrees) or an absolute path for one
	// shared root. Keep it identical across profiles — the gitignore entry
	// renders into the shared repository.
	Worktrees string `json:"worktrees,omitempty"`

	// PrivateRepo is the encrypted private dotfiles repository, stored like
	// Repo: relative to ReposDir, absolute (tilde-folded) only when outside
	// it. Empty means the machine keeps no private repository. Only the
	// repository's path lives here — its contents stay age-encrypted in the
	// repository itself.
	PrivateRepo string `json:"privateRepo"`
}

// DefaultWorktrees is the repo-relative directory agent worktrees live in
// when init is not told otherwise.
const DefaultWorktrees = ".worktrees"

// SaveAnswers writes a to profileDir/profile.json.
func SaveAnswers(profileDir string, a Answers) error {
	data, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		return fmt.Errorf("encode answers: %w", err)
	}
	return cli.AtomicWriteFile(filepath.Join(profileDir, AnswersFile), append(data, '\n'), 0o644)
}

// AnswerKeys reports which questions a stored answers document actually
// answers — one field per wizard question, true when the document carries
// that question's key with a non-null value. It is how a --yes re-run tells
// an answered question (reusable, even when the answer is a falsy "no" or an
// empty pick) from one the document predates and must still ask.
type AnswerKeys struct {
	ReposDir       bool
	Repo           bool
	AddOns         bool
	Agents         bool
	ImportPackages bool
	SecurityKeys   bool
	Harden         bool
	Marketplace    bool
	HushLogin      bool
	MacOSDefaults  bool
	DockApps       bool
	PIV            bool
	PrivateRepo    bool
}

// answerKeys derives AnswerKeys from a document's raw keys; the literals here
// mirror the json tags on Answers. A null value does not count as an answer —
// null is how a never-collected slice serializes.
func answerKeys(raw map[string]json.RawMessage) AnswerKeys {
	has := func(key string) bool {
		v, ok := raw[key]
		return ok && string(v) != "null"
	}
	return AnswerKeys{
		ReposDir:       has("reposDir"),
		Repo:           has("repo"),
		AddOns:         has("addons"),
		Agents:         has("agents"),
		ImportPackages: has("importPackages") || has(legacyImportKey),
		SecurityKeys:   has("securityKeys"),
		Harden:         has("harden"),
		Marketplace:    has("marketplace"),
		HushLogin:      has("hushLogin"),
		MacOSDefaults:  has("macosDefaults"),
		DockApps:       has("dockApps"),
		PIV:            has("piv"),
		PrivateRepo:    has("privateRepo"),
	}
}

// LoadAnswers reads a profile directory's answers: profile.json, falling
// back to the legacy dotty.json when profile.json is missing or carries only
// metadata — a pre-merge profile has both documents, and a profile made
// before init owned their creation has metadata alone. A profile with no
// answers anywhere reports
// fs.ErrNotExist like a missing file — there is nothing to seed a re-run
// with.
func LoadAnswers(profileDir string) (Answers, error) {
	a, _, err := LoadAnswersWithKeys(profileDir)
	return a, err
}

// LoadAnswersWithKeys is LoadAnswers plus the set of questions the stored
// document answers, for callers that must tell a reusable answer from a
// question the document never saw.
func LoadAnswersWithKeys(profileDir string) (Answers, AnswerKeys, error) {
	a, keys, err := readAnswers(filepath.Join(profileDir, AnswersFile))
	if err == nil && a.ProfileName != "" {
		return a, keys, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Answers{}, AnswerKeys{}, err
	}
	legacy, legacyKeys, legacyErr := readAnswers(filepath.Join(profileDir, legacyAnswersFile))
	if legacyErr == nil && legacy.ProfileName != "" {
		return legacy, legacyKeys, nil
	}
	if legacyErr != nil && !errors.Is(legacyErr, fs.ErrNotExist) {
		return Answers{}, AnswerKeys{}, legacyErr
	}
	return Answers{}, AnswerKeys{}, fmt.Errorf("%s carries no answers: %w",
		filepath.Join(profileDir, AnswersFile), fs.ErrNotExist)
}

// legacyImportKey is the json key ImportPackages had while packages came
// from a Brewfile.
const legacyImportKey = "dumpBrews"

// readAnswers parses one answers document. A document from before packages
// moved to mise answers the import question under the legacy key; its value
// carries over. One that predates the hush-login question takes its
// default, on.
func readAnswers(path string) (Answers, AnswerKeys, error) {
	var a Answers
	data, err := os.ReadFile(path)
	if err != nil {
		return a, AnswerKeys{}, fmt.Errorf("read answers: %w", err)
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return a, AnswerKeys{}, fmt.Errorf("parse %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return a, AnswerKeys{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if legacy, ok := raw[legacyImportKey]; ok {
		if _, current := raw["importPackages"]; !current {
			_ = json.Unmarshal(legacy, &a.ImportPackages) // a malformed legacy value reads as unanswered
		}
	}
	keys := answerKeys(raw)
	if !keys.HushLogin {
		a.HushLogin = true
	}
	a.AddOns = CanonicalAddOns(a.AddOns)
	return a, keys, nil
}

// renamedAddOns maps retired add-on names onto the add-ons that replaced
// them, so a stored profile or an --addons flag written before a rename
// still selects the component. k9s grew into kubernetes when the add-on
// took on kubectl, flux and the colima/docker toolchain.
var renamedAddOns = map[string]string{"k9s": "kubernetes"}

// CanonicalAddOns rewrites retired add-on names to their replacements,
// dropping any duplicate the rewrite creates. Order and nilness are kept —
// a nil slice is a never-asked question, an empty one an answered "none".
func CanonicalAddOns(names []string) []string {
	if names == nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if renamed, ok := renamedAddOns[n]; ok {
			n = renamed
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// TildePath folds a home prefix into ~ so the path stores portably in
// answers that travel across machines with different homes.
func TildePath(path, home string) string {
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return path
}

// ExpandTilde resolves a ~-prefixed stored path against home.
func ExpandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}

// shellHome rewrites a path under home to its ${HOME}-relative shell form,
// so rendered shell files carry no machine-specific prefix.
func shellHome(path, home string) string {
	if path == home {
		return "${HOME}"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "${HOME}/" + rest
	}
	return path
}

// Vars are the substitutions available to templated files.
type Vars struct {
	// Version is the dotty release rendering the template — stamped into the
	// repository's .dotty-version marker so future upgrades can tell what a
	// repo was rendered with.
	Version string

	Home        string
	ProfileName string
	ReposDir    string // absolute — sandbox configs need real paths
	// ReposDirShell is the ${HOME}-relative form for rendered shell files,
	// so a machine class shares them across homes.
	ReposDirShell string
	DotfilesDir   string
	// BrewPrefix is Homebrew's prefix on the OS and architecture rendering
	// the template; the core fragment points the login shell at its zsh.
	BrewPrefix string

	SecurityKeys bool
	Harden       bool
	Marketplace  bool

	HasNvim       bool
	HasLsd        bool
	HasYazi       bool
	HasCodex      bool
	HasClaude     bool
	HasOpenClaude bool
	HasGrok       bool

	// OpenClaudeSettings is scoped to rendering OpenClaude's settings from
	// the shared Claude template; provider selection owns its model default.
	OpenClaudeSettings bool

	// DockGhostty, DockClaude and DockChatGPT pin the optional apps in the
	// dock defaults group's fragment. An agent's app needs the agent
	// selected, so a stale pick drops out with its agent.
	DockGhostty bool
	DockClaude  bool
	DockChatGPT bool

	// Worktrees is the raw answer; the derived forms feed different template
	// spots. WorktreesRoot is a permission-rule glob (**/<name> or the
	// absolute root); WorktreesAbs is the absolute root, or empty when
	// worktrees are repo-relative (already covered by the ReposDir sandbox
	// grants).
	Worktrees     string
	WorktreesRoot string
	WorktreesAbs  string
}

// BrewPrefix is Homebrew's default prefix for goos/goarch: /opt/homebrew on
// Apple silicon, /usr/local on Intel Macs, and the shared linuxbrew prefix
// on Linux.
func BrewPrefix(goos, goarch string) string {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return "/opt/homebrew"
	case goos == "darwin":
		return "/usr/local"
	default:
		return "/home/linuxbrew/.linuxbrew"
	}
}

// NewVars derives template variables from the answers plus the machine facts
// only the caller knows.
func NewVars(a Answers, home, dotfilesDir string) Vars {
	reposDir := ExpandTilde(a.ReposDir, home)
	v := Vars{
		Version:       version.Version,
		Home:          home,
		ProfileName:   a.ProfileName,
		ReposDir:      reposDir,
		ReposDirShell: shellHome(reposDir, home),
		DotfilesDir:   dotfilesDir,
		BrewPrefix:    BrewPrefix(runtime.GOOS, runtime.GOARCH),
		SecurityKeys:  a.SecurityKeys,
		Harden:        a.Harden,
		Marketplace:   a.Marketplace,
		HasNvim:       slices.Contains(a.AddOns, "nvim"),
		HasLsd:        slices.Contains(a.AddOns, "lsd"),
		HasYazi:       slices.Contains(a.AddOns, "yazi"),
		HasCodex:      slices.Contains(a.Agents, "codex"),
		HasClaude:     slices.Contains(a.Agents, "claude-code"),
		HasOpenClaude: slices.Contains(a.Agents, "openclaude"),
		HasGrok:       slices.Contains(a.Agents, "grok"),
	}
	dock := func(app string) bool { return a.DockApps == nil || slices.Contains(a.DockApps, app) }
	v.DockGhostty = dock("ghostty")
	v.DockClaude = v.HasClaude && dock("claude-code")
	v.DockChatGPT = v.HasCodex && dock("codex")
	v.Worktrees = a.Worktrees
	if v.Worktrees == "" {
		v.Worktrees = DefaultWorktrees
	}
	if strings.HasPrefix(v.Worktrees, "~") || filepath.IsAbs(v.Worktrees) {
		abs := v.Worktrees
		if strings.HasPrefix(abs, "~/") {
			abs = filepath.Join(home, abs[2:])
		}
		v.WorktreesAbs = abs
		v.WorktreesRoot = abs
	} else {
		v.WorktreesRoot = "**/" + v.Worktrees
	}
	return v
}
