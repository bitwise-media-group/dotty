// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package brewfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeRunner records argv and serves scripted results. Output results are
// consumed in call order; Run errors likewise.
type fakeRunner struct {
	calls      [][]string
	outputs    [][]byte
	outputErrs []error
	runErrs    []error
	outputN    int
	runN       int
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	var out []byte
	var err error
	if f.outputN < len(f.outputs) {
		out = f.outputs[f.outputN]
	}
	if f.outputN < len(f.outputErrs) {
		err = f.outputErrs[f.outputN]
	}
	f.outputN++
	return out, err
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	var err error
	if f.runN < len(f.runErrs) {
		err = f.runErrs[f.runN]
	}
	f.runN++
	return err
}

func (f *fakeRunner) argv(i int) string { return strings.Join(f.calls[i], " ") }

func yes(string) (bool, error) { return true, nil }
func no(string) (bool, error)  { return false, nil }

const emptyTrust = `{"taps":[],"formulae":[],"casks":[],"commands":[]}`

// seedBrewfile writes content to a Brewfile in a fresh temp dir and returns
// its path, so post-Add assertions can inspect what markTrusted wrote.
func seedBrewfile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Brewfile")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed Brewfile: %v", err)
	}
	return path
}

func readBrewfile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Brewfile: %v", err)
	}
	return string(data)
}

func assertCalls(t *testing.T, r *fakeRunner, want []string) {
	t.Helper()
	if len(r.calls) != len(want) {
		t.Fatalf("calls = %v, want %d calls %v", r.calls, len(want), want)
	}
	for i, w := range want {
		if r.argv(i) != w {
			t.Errorf("call %d = %q, want %q", i, r.argv(i), w)
		}
	}
}

func TestAdd(t *testing.T) {
	ctx := context.Background()

	// A path that does not exist skips the `brew bundle list` read entirely —
	// brew creates the file on add.
	t.Run("plain formula skips trust and adds then installs", func(t *testing.T) {
		r := &fakeRunner{}
		res, err := Add(ctx, r, "/p/Brewfile", KindFormula, []string{"jq", "ripgrep"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle add --file=/p/Brewfile --formula jq ripgrep",
			"brew bundle install --file=/p/Brewfile",
		})
		if res.Skipped != nil || res.Unmarked != nil {
			t.Errorf("result = %+v, want empty", res)
		}
	})

	t.Run("untrusted tap-qualified formula is trusted, added, and marked", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"acme/tap/widget\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(emptyTrust), []byte("")}}
		res, err := Add(ctx, r, path, KindFormula, []string{"acme/tap/widget"}, yes)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --formula",
			"brew trust --json v1",
			"brew trust --formula acme/tap/widget",
			"brew tap",
			"brew tap acme/tap",
			"brew bundle add --file=" + path + " --formula acme/tap/widget",
			"brew bundle install --file=" + path,
		})
		if got, want := readBrewfile(t, path), "brew \"acme/tap/widget\", trusted: true\n"; got != want {
			t.Errorf("Brewfile = %q, want %q", got, want)
		}
		if res.Unmarked != nil {
			t.Errorf("Unmarked = %v, want none", res.Unmarked)
		}
	})

	t.Run("already-trusted name skips the trust write", func(t *testing.T) {
		// The verbatim mixed-case argument must match the store's normalised
		// entry and still be found on the Brewfile line brew wrote verbatim.
		path := seedBrewfile(t, "brew \"Acme/homebrew-tap/Widget\"\n")
		trusted := `{"taps":[],"formulae":["acme/tap/widget"],"casks":[],"commands":[]}`
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(trusted), []byte("acme/tap\n")}}
		res, err := Add(ctx, r, path, KindFormula, []string{"Acme/homebrew-tap/Widget"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		for _, call := range r.calls {
			if slices.Contains(call, "--formula") && call[1] == "trust" {
				t.Errorf("unexpected trust write: %v", call)
			}
		}
		if got, want := readBrewfile(t, path), "brew \"Acme/homebrew-tap/Widget\", trusted: true\n"; got != want {
			t.Errorf("Brewfile = %q, want %q", got, want)
		}
		if res.Unmarked != nil {
			t.Errorf("Unmarked = %v, want none", res.Unmarked)
		}
	})

	t.Run("declined trust aborts before any write", func(t *testing.T) {
		r := &fakeRunner{outputs: [][]byte{[]byte(emptyTrust)}}
		_, err := Add(ctx, r, "/p/Brewfile", KindCask, []string{"acme/tap/widget"}, no)
		if err == nil {
			t.Fatal("Add() error = nil, want declined-trust failure")
		}
		if len(r.calls) != 1 { // only the trust-store read
			t.Errorf("calls = %v, want only the trust read", r.calls)
		}
	})

	t.Run("non-trustable kinds never consult trust", func(t *testing.T) {
		r := &fakeRunner{}
		if _, err := Add(ctx, r, "/p/Brewfile", KindNPM, []string{"acme/scope/pkg"}, no); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		if r.argv(0) != "brew bundle add --file=/p/Brewfile --npm acme/scope/pkg" {
			t.Errorf("call 0 = %q", r.argv(0))
		}
	})

}

func TestAddErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("entry line not found is reported, not fatal", func(t *testing.T) {
		path := seedBrewfile(t, "# placeholder\n")
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(emptyTrust)}}
		res, err := Add(ctx, r, path, KindFormula, []string{"acme/tap/widget"}, yes)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		if !slices.Equal(res.Unmarked, []string{"acme/tap/widget"}) {
			t.Errorf("Unmarked = %v, want [acme/tap/widget]", res.Unmarked)
		}
		if got := readBrewfile(t, path); got != "# placeholder\n" {
			t.Errorf("Brewfile = %q, want untouched", got)
		}
	})

	t.Run("list failure is a real error", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"jq\"\n")
		r := &fakeRunner{outputErrs: []error{errors.New("brew exploded")}}
		_, err := Add(ctx, r, path, KindFormula, []string{"jq"}, no)
		if err == nil {
			t.Fatal("Add() error = nil, want list failure")
		}
		if !strings.Contains(err.Error(), "list formula entries") {
			t.Errorf("error = %v, want list context", err)
		}
		if len(r.calls) != 1 {
			t.Errorf("calls = %v, want the list read only", r.calls)
		}
	})
}

func TestAddDedupe(t *testing.T) {
	ctx := context.Background()

	t.Run("names already in the Brewfile are skipped", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"jq\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte("jq\n")}}
		res, err := Add(ctx, r, path, KindFormula, []string{"jq", "ripgrep"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --formula",
			"brew bundle add --file=" + path + " --formula ripgrep",
			"brew bundle install --file=" + path,
		})
		if !slices.Equal(res.Skipped, []string{"jq"}) {
			t.Errorf("Skipped = %v, want [jq]", res.Skipped)
		}
	})

	t.Run("all names present still installs", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"jq\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte("jq\n")}}
		res, err := Add(ctx, r, path, KindFormula, []string{"jq"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --formula",
			"brew bundle install --file=" + path,
		})
		if !slices.Equal(res.Skipped, []string{"jq"}) {
			t.Errorf("Skipped = %v, want [jq]", res.Skipped)
		}
	})

	t.Run("repeated arguments are added once", func(t *testing.T) {
		r := &fakeRunner{}
		res, err := Add(ctx, r, "/p/Brewfile", KindFormula, []string{"ripgrep", "ripgrep"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle add --file=/p/Brewfile --formula ripgrep",
			"brew bundle install --file=/p/Brewfile",
		})
		if !slices.Equal(res.Skipped, []string{"ripgrep"}) {
			t.Errorf("Skipped = %v, want [ripgrep]", res.Skipped)
		}
	})

	t.Run("canonical forms dedupe against listed entries", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"acme/tap/widget\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte("acme/tap/widget\n")}}
		res, err := Add(ctx, r, path, KindFormula, []string{"Acme/homebrew-tap/Widget"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{ // skipped names bypass the trust flow too
			"brew bundle list --file=" + path + " --formula",
			"brew bundle install --file=" + path,
		})
		if !slices.Equal(res.Skipped, []string{"Acme/homebrew-tap/Widget"}) {
			t.Errorf("Skipped = %v", res.Skipped)
		}
	})

	t.Run("casks dedupe on their short token", func(t *testing.T) {
		path := seedBrewfile(t, "cask \"widget\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte("widget\n")}}
		res, err := Add(ctx, r, path, KindCask, []string{"acme/tap/widget"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --cask",
			"brew bundle install --file=" + path,
		})
		if !slices.Equal(res.Skipped, []string{"acme/tap/widget"}) {
			t.Errorf("Skipped = %v", res.Skipped)
		}
	})
}

func TestAddEnsureTaps(t *testing.T) {
	ctx := context.Background()

	t.Run("already-installed taps are not re-tapped", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"Acme/homebrew-tap/Widget\"\n")
		trusted := `{"taps":[],"formulae":["acme/tap/widget"],"casks":[],"commands":[]}`
		// The mixed-case homebrew- prefixed argument must match brew's
		// canonical tap listing.
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(trusted), []byte("acme/tap\nother/tap\n")}}
		if _, err := Add(ctx, r, path, KindFormula, []string{"Acme/homebrew-tap/Widget"}, no); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --formula",
			"brew trust --json v1",
			"brew tap",
			"brew bundle add --file=" + path + " --formula Acme/homebrew-tap/Widget",
			"brew bundle install --file=" + path,
		})
	})

	t.Run("a tap shared by several names is tapped once", func(t *testing.T) {
		path := seedBrewfile(t, "brew \"acme/tap/widget\"\nbrew \"acme/tap/gadget\"\nbrew \"jq\"\n")
		trusted := `{"taps":[],"formulae":["acme/tap/widget","acme/tap/gadget"],"casks":[],"commands":[]}`
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(trusted), []byte(trusted), []byte("")}}
		res, err := Add(ctx, r, path, KindFormula,
			[]string{"acme/tap/widget", "acme/tap/gadget", "jq"}, no)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --formula",
			"brew trust --json v1",
			"brew trust --json v1",
			"brew tap",
			"brew tap acme/tap",
			"brew bundle add --file=" + path + " --formula acme/tap/widget acme/tap/gadget jq",
			"brew bundle install --file=" + path,
		})
		if res.Unmarked != nil {
			t.Errorf("Unmarked = %v, want none", res.Unmarked)
		}
	})

	t.Run("plain and official names skip the tap check", func(t *testing.T) {
		r := &fakeRunner{}
		if _, err := Add(ctx, r, "/p/Brewfile", KindFormula, []string{"homebrew/core/jq"}, no); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle add --file=/p/Brewfile --formula homebrew/core/jq",
			"brew bundle install --file=/p/Brewfile",
		})
	})

	t.Run("tap install failure aborts before the Brewfile is touched", func(t *testing.T) {
		trusted := `{"taps":[],"formulae":[],"casks":["acme/tap/widget"],"commands":[]}`
		r := &fakeRunner{
			outputs: [][]byte{[]byte(trusted), []byte("")},
			runErrs: []error{errors.New("clone failed")},
		}
		_, err := Add(ctx, r, "/p/Brewfile", KindCask, []string{"acme/tap/widget"}, no)
		if err == nil {
			t.Fatal("Add() error = nil, want tap failure")
		}
		if !strings.Contains(err.Error(), "tap acme/tap") {
			t.Errorf("error = %v, want tap context", err)
		}
		assertCalls(t, r, []string{
			"brew trust --json v1",
			"brew tap",
			"brew tap acme/tap",
		})
	})
}

func TestAddTaps(t *testing.T) {
	ctx := context.Background()

	t.Run("taps are trusted and marked", func(t *testing.T) {
		path := seedBrewfile(t, "tap \"fluxcd/tap\"\n")
		r := &fakeRunner{outputs: [][]byte{[]byte(""), []byte(emptyTrust)}}
		res, err := Add(ctx, r, path, KindTap, []string{"fluxcd/tap"}, yes)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle list --file=" + path + " --tap",
			"brew trust --json v1",
			"brew trust --tap fluxcd/tap",
			"brew bundle add --file=" + path + " --tap fluxcd/tap",
			"brew bundle install --file=" + path,
		})
		if got, want := readBrewfile(t, path), "tap \"fluxcd/tap\", trusted: true\n"; got != want {
			t.Errorf("Brewfile = %q, want %q", got, want)
		}
		if res.Unmarked != nil {
			t.Errorf("Unmarked = %v, want none", res.Unmarked)
		}
	})

	t.Run("official homebrew names skip trust", func(t *testing.T) {
		r := &fakeRunner{}
		if _, err := Add(ctx, r, "/p/Brewfile", KindTap, []string{"homebrew/services"}, no); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		assertCalls(t, r, []string{
			"brew bundle add --file=/p/Brewfile --tap homebrew/services",
			"brew bundle install --file=/p/Brewfile",
		})
	})
}

func TestUpgrade(t *testing.T) {
	r := &fakeRunner{}
	if err := Upgrade(context.Background(), r, "/p/Brewfile"); err != nil {
		t.Fatalf("Upgrade() error: %v", err)
	}
	if want := "brew bundle install --file=/p/Brewfile --upgrade"; r.argv(0) != want {
		t.Errorf("argv = %q, want %q", r.argv(0), want)
	}
}

func TestSync(t *testing.T) {
	ctx := context.Background()
	installArgv := "brew bundle install --file=/p/Brewfile --force --force-cleanup --upgrade --zap"

	t.Run("force skips the dry-run", func(t *testing.T) {
		r := &fakeRunner{}
		if err := Sync(ctx, r, "/p/Brewfile", true, nil); err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		if len(r.calls) != 1 || r.argv(0) != installArgv {
			t.Errorf("calls = %v", r.calls)
		}
	})

	t.Run("clean dry-run proceeds without confirmation", func(t *testing.T) {
		r := &fakeRunner{}
		confirmed := false
		err := Sync(ctx, r, "/p/Brewfile", false, func([]string) (bool, error) { confirmed = true; return true, nil })
		if err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		if confirmed {
			t.Error("confirm called with nothing to remove")
		}
		if r.argv(0) != "brew bundle cleanup --file=/p/Brewfile" {
			t.Errorf("dry-run argv = %q", r.argv(0))
		}
		if r.argv(1) != installArgv {
			t.Errorf("install argv = %q", r.argv(1))
		}
	})

	t.Run("removals are confirmed before installing", func(t *testing.T) {
		r := &fakeRunner{
			outputs:    [][]byte{[]byte("Would uninstall formulae:\njq\n")},
			outputErrs: []error{errors.New("exit status 1")},
		}
		var got []string
		err := Sync(ctx, r, "/p/Brewfile", false, func(removals []string) (bool, error) {
			got = removals
			return true, nil
		})
		if err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		if !slices.Contains(got, "jq") {
			t.Errorf("removals = %v, want to contain jq", got)
		}
		if r.argv(len(r.calls)-1) != installArgv {
			t.Errorf("last call = %q, want install", r.argv(len(r.calls)-1))
		}
	})

	t.Run("declining the removals aborts", func(t *testing.T) {
		r := &fakeRunner{
			outputs:    [][]byte{[]byte("Would uninstall formulae:\njq\n")},
			outputErrs: []error{errors.New("exit status 1")},
		}
		if err := Sync(ctx, r, "/p/Brewfile", false, func([]string) (bool, error) { return false, nil }); err != nil {
			t.Fatalf("Sync() error: %v", err)
		}
		if len(r.calls) != 1 {
			t.Errorf("calls = %v, want dry-run only", r.calls)
		}
	})

	t.Run("dry-run failure with no output is a real error", func(t *testing.T) {
		r := &fakeRunner{outputErrs: []error{errors.New("brew exploded")}}
		if err := Sync(ctx, r, "/p/Brewfile", false, nil); err == nil {
			t.Fatal("Sync() error = nil, want failure")
		}
	})
}

func TestDump(t *testing.T) {
	ctx := context.Background()

	t.Run("default kinds per DESIGN", func(t *testing.T) {
		r := &fakeRunner{}
		if err := Dump(ctx, r, "/p/Brewfile", false, false); err != nil {
			t.Fatalf("Dump() error: %v", err)
		}
		want := "brew bundle dump --file=/p/Brewfile --formula --cask --mas --flatpak"
		if r.argv(0) != want {
			t.Errorf("argv = %q, want %q", r.argv(0), want)
		}
	})

	t.Run("all spells out every type flag", func(t *testing.T) {
		r := &fakeRunner{}
		if err := Dump(ctx, r, "/p/Brewfile", true, true); err != nil {
			t.Fatalf("Dump() error: %v", err)
		}
		argv := r.argv(0)
		for _, flag := range []string{"--tap", "--vscode", "--go", "--cargo", "--uv", "--krew", "--npm", "--force"} {
			if !strings.Contains(argv, flag) {
				t.Errorf("argv %q missing %s", argv, flag)
			}
		}
	})
}

func TestKindFlags(t *testing.T) {
	for kind, want := range map[Kind]string{KindFormula: "--formula", KindFlatpak: "--flatpak", "mas": "--mas"} {
		if got := kind.flag(); got != want {
			t.Errorf("flag(%s) = %q, want %q", kind, got, want)
		}
	}
}
