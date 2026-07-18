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
	ReposDir  string   `json:"reposDir"`
	Repo      string   `json:"repo"`
	AddOns    []string `json:"addons,omitzero"`
	Agents    []string `json:"agents,omitzero"`
	DumpBrews bool     `json:"dumpBrews"`

	SecurityKeys bool `json:"securityKeys"`
	Harden       bool `json:"harden"`
	Marketplace  bool `json:"marketplace"`

	MacOSDefaults []string `json:"macosDefaults,omitzero"`
	Wallpaper     string   `json:"wallpaper"`
	PIV           bool     `json:"piv"`

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
	ReposDir      bool
	Repo          bool
	AddOns        bool
	Agents        bool
	DumpBrews     bool
	SecurityKeys  bool
	Harden        bool
	Marketplace   bool
	MacOSDefaults bool
	Wallpaper     bool
	PIV           bool
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
		ReposDir:      has("reposDir"),
		Repo:          has("repo"),
		AddOns:        has("addons"),
		Agents:        has("agents"),
		DumpBrews:     has("dumpBrews"),
		SecurityKeys:  has("securityKeys"),
		Harden:        has("harden"),
		Marketplace:   has("marketplace"),
		MacOSDefaults: has("macosDefaults"),
		Wallpaper:     has("wallpaper"),
		PIV:           has("piv"),
	}
}

// LoadAnswers reads a profile directory's answers: profile.json, falling
// back to the legacy dotty.json when profile.json is missing or carries only
// metadata — a pre-merge profile has both documents, and `dotty profile new`
// writes metadata alone. A profile with no answers anywhere reports
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

// readAnswers parses one answers document.
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
	return a, answerKeys(raw), nil
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

	SecurityKeys bool
	Harden       bool
	Marketplace  bool

	HasNvim   bool
	HasLsd    bool
	HasYazi   bool
	HasCodex  bool
	HasClaude bool
	HasGrok   bool

	// Worktrees is the raw answer; the derived forms feed different template
	// spots. WorktreesRoot is a permission-rule glob (**/<name> or the
	// absolute root); WorktreesAbs is the absolute root, or empty when
	// worktrees are repo-relative (already covered by the ReposDir sandbox
	// grants).
	Worktrees     string
	WorktreesRoot string
	WorktreesAbs  string
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
		SecurityKeys:  a.SecurityKeys,
		Harden:        a.Harden,
		Marketplace:   a.Marketplace,
		HasNvim:       slices.Contains(a.AddOns, "nvim"),
		HasLsd:        slices.Contains(a.AddOns, "lsd"),
		HasYazi:       slices.Contains(a.AddOns, "yazi"),
		HasCodex:      slices.Contains(a.Agents, "codex"),
		HasClaude:     slices.Contains(a.Agents, "claude-code"),
		HasGrok:       slices.Contains(a.Agents, "grok"),
	}
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
