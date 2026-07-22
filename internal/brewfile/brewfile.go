// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package brewfile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// Runner executes brew on behalf of this package; tests substitute a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Kind is a brew bundle entry type. Its string form doubles as the brew CLI
// flag name.
type Kind string

// The entry types `brew bundle add` accepts.
const (
	KindFormula Kind = "formula"
	KindCask    Kind = "cask"
	KindTap     Kind = "tap"
	KindVSCode  Kind = "vscode"
	KindGo      Kind = "go"
	KindCargo   Kind = "cargo"
	KindUV      Kind = "uv"
	KindFlatpak Kind = "flatpak"
	KindKrew    Kind = "krew"
	KindNPM     Kind = "npm"
)

// Trustable reports whether `brew trust` applies to this kind — Homebrew
// trusts only taps, formulae, and casks.
func (k Kind) Trustable() bool {
	return k == KindFormula || k == KindCask || k == KindTap
}

func (k Kind) flag() string { return "--" + string(k) }

// AddResult reports what Add did beyond adding and installing: names already
// recorded in the Brewfile were skipped rather than duplicated, and names
// whose freshly added entry line could not be rewritten with `trusted: true`
// need the attribute added by hand.
type AddResult struct {
	Skipped  []string
	Unmarked []string
}

// Add records names in the Brewfile and installs the bundle. Names brew's own
// parser already reports for the kind are skipped — `brew bundle add` appends
// blindly. Trust-gated names (see NeedsTrust) go through the `brew trust`
// flow first: when one is untrusted, confirmTrust decides whether to trust
// it; declining aborts before anything is written. Newly added trust-gated
// entries are then marked `trusted: true` in the Brewfile itself, because
// `brew bundle install --force-cleanup` (used by Sync) resets Homebrew's
// trust store to exactly what the Brewfile declares. Taps referenced by
// tap-qualified formula and cask names are installed after trusting and
// before the add — see ensureTaps. Install always runs,
// even when every name was skipped — it converges a machine where an entry is
// recorded but not installed.
func Add(ctx context.Context, r Runner, path string, kind Kind, names []string,
	confirmTrust func(name string) (bool, error),
) (AddResult, error) {
	var res AddResult
	present, err := listEntries(ctx, r, path, kind)
	if err != nil {
		return res, err
	}
	var toAdd []string
	for _, name := range names {
		canonical := canonicalName(kind, name)
		if present[canonical] {
			res.Skipped = append(res.Skipped, name)
			continue
		}
		present[canonical] = true // also dedupes repeats within one invocation
		toAdd = append(toAdd, name)
	}

	if len(toAdd) > 0 {
		var trustNames []string
		for _, name := range toAdd {
			if !NeedsTrust(kind, name) {
				continue
			}
			trustNames = append(trustNames, name)
			trusted, err := IsTrusted(ctx, r, kind, name)
			if err != nil {
				return res, err
			}
			if trusted {
				continue
			}
			ok, err := confirmTrust(name)
			if err != nil {
				return res, err
			}
			if !ok {
				return res, fmt.Errorf("%s %q is not trusted (declined)", kind, name)
			}
			if err := Trust(ctx, r, kind, name); err != nil {
				return res, err
			}
		}

		if err := ensureTaps(ctx, r, kind, toAdd); err != nil {
			return res, err
		}

		addArgs := append([]string{"bundle", "add", "--file=" + path, kind.flag()}, toAdd...)
		if err := r.Run(ctx, "brew", addArgs...); err != nil {
			return res, err
		}
		if res.Unmarked, err = markTrusted(path, kind, trustNames); err != nil {
			return res, err
		}
	}
	return res, r.Run(ctx, "brew", "bundle", "install", "--file="+path)
}

// ensureTaps installs the third-party taps that tap-qualified formula and
// cask names reference. Homebrew 6 stopped installing taps implicitly when a
// fully-qualified name is loaded ("Stop implicit tap installation"), and
// `brew bundle add` loads each formula or cask to read its description — so
// an untapped name fails there before the Brewfile is touched. `brew bundle
// install` taps its own entries, so only the add step needs this. Runs after
// the trust flow because installing a tap can load its packages, which trips
// Homebrew's trust gate.
func ensureTaps(ctx context.Context, r Runner, kind Kind, names []string) error {
	if kind != KindFormula && kind != KindCask {
		return nil
	}
	var taps []string
	for _, name := range names {
		if !NeedsTrust(kind, name) {
			continue
		}
		parts := strings.SplitN(name, "/", 3)
		tap := canonicalName(KindTap, parts[0]+"/"+parts[1])
		if !slices.Contains(taps, tap) {
			taps = append(taps, tap)
		}
	}
	if len(taps) == 0 {
		return nil
	}
	out, err := r.Output(ctx, "brew", "tap")
	if err != nil {
		return fmt.Errorf("list installed taps: %w", err)
	}
	installed := nonEmptyLines(out)
	for _, tap := range taps {
		if slices.Contains(installed, tap) {
			continue
		}
		if err := r.Run(ctx, "brew", "tap", tap); err != nil {
			return fmt.Errorf("tap %s: %w", tap, err)
		}
	}
	return nil
}

// listEntries returns the canonical names of kind already recorded at path,
// parsed by brew itself. A missing Brewfile is an empty set — `brew bundle
// add` creates it.
func listEntries(ctx context.Context, r Runner, path string, kind Kind) (map[string]bool, error) {
	present := make(map[string]bool)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return present, nil
	}
	out, err := r.Output(ctx, "brew", "bundle", "list", "--file="+path, kind.flag())
	if err != nil {
		return nil, fmt.Errorf("list %s entries in %s: %w", kind, path, err)
	}
	for _, line := range nonEmptyLines(out) {
		present[canonicalName(kind, line)] = true
	}
	return present, nil
}

// Upgrade installs and upgrades everything in the Brewfile without removing
// anything. brew bundle install does not clean up unless asked, so no flag is
// needed to preserve unlisted brews (and the former --no-cleanup flag has been
// removed from Homebrew).
func Upgrade(ctx context.Context, r Runner, path string) error {
	return r.Run(ctx, "brew", "bundle", "install", "--file="+path, "--upgrade")
}

// Sync makes the machine match the Brewfile exactly, removing brews that are
// not listed. Unless force is set, a cleanup dry-run lists the would-be
// removals first and confirm decides whether to proceed; returning false
// aborts with no changes.
func Sync(ctx context.Context, r Runner, path string, force bool, confirm func(removals []string) (bool, error)) error {
	if !force {
		removals, err := cleanupDryRun(ctx, r, path)
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
		}
	}
	return r.Run(ctx, "brew", "bundle", "install", "--file="+path,
		"--force", "--force-cleanup", "--upgrade", "--zap")
}

// cleanupDryRun runs `brew bundle cleanup` without --force, which exits
// non-zero and prints the would-be removals when anything would go. A failure
// with no stdout is a real error, not a removal listing.
func cleanupDryRun(ctx context.Context, r Runner, path string) ([]string, error) {
	out, err := r.Output(ctx, "brew", "bundle", "cleanup", "--file="+path)
	if err == nil {
		return nil, nil
	}
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return nil, fmt.Errorf("check for removable brews: %w", err)
	}
	return lines, nil
}

// dumpKinds is what `dotty brewfile dump` writes by default, per DESIGN.
var dumpKinds = []Kind{KindFormula, KindCask, "mas", KindFlatpak}

// allDumpKinds is every positive type flag `brew bundle dump` accepts —
// Homebrew has no --all flag, so dotty's --all spells them out.
var allDumpKinds = []Kind{KindFormula, KindCask, KindTap, "mas", KindFlatpak,
	KindVSCode, KindGo, KindCargo, KindUV, KindKrew, KindNPM}

// Dump snapshots the installed brews into the Brewfile. force overwrites an
// existing file; deciding whether that is wanted is the caller's job.
func Dump(ctx context.Context, r Runner, path string, all, force bool) error {
	args := []string{"bundle", "dump", "--file=" + path}
	kinds := dumpKinds
	if all {
		kinds = allDumpKinds
	}
	for _, k := range kinds {
		args = append(args, k.flag())
	}
	if force {
		args = append(args, "--force")
	}
	return r.Run(ctx, "brew", args...)
}

func nonEmptyLines(out []byte) []string {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
