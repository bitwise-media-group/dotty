// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner satisfies brewfile.Runner, recording brew invocations.
type fakeRunner struct {
	calls [][]string
	err   error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.err
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return nil, f.err
}

func activateForTest(t *testing.T, configDir, name string) (string, *fakeRunner, error) {
	t.Helper()
	r := &fakeRunner{}
	dir, err := Activate(context.Background(), r, configDir, name)
	return dir, r, err
}

func TestActivate(t *testing.T) {
	t.Run("points the symlink at the profile and dumps a Brewfile", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, "work", ""); err != nil {
			t.Fatal(err)
		}
		got, r, err := activateForTest(t, dir, "work")
		if err != nil {
			t.Fatalf("Activate() error: %v", err)
		}
		if want := Dir(dir, "work"); got != want {
			t.Errorf("Activate() dir = %q, want %q", got, want)
		}

		target, err := os.Readlink(filepath.Join(dir, "active-profile"))
		if err != nil {
			t.Fatalf("readlink: %v", err)
		}
		if target != "work" {
			t.Errorf("symlink target = %q, want relative %q", target, "work")
		}

		active, err := ActiveDir(dir)
		if err != nil || active != Dir(dir, "work") {
			t.Errorf("ActiveDir() = %q, %v", active, err)
		}
		name, err := ActiveName(dir)
		if err != nil || name != "work" {
			t.Errorf("ActiveName() = %q, %v", name, err)
		}

		// No Brewfile existed, so a dump must have been requested.
		if len(r.calls) != 1 || !strings.Contains(strings.Join(r.calls[0], " "), "bundle dump") {
			t.Errorf("brew calls = %v, want one bundle dump", r.calls)
		}
	})

	t.Run("existing Brewfile is not re-dumped", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, "work", ""); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(BrewfilePath(Dir(dir, "work")), []byte("brew \"jq\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, r, err := activateForTest(t, dir, "work")
		if err != nil {
			t.Fatalf("Activate() error: %v", err)
		}
		if len(r.calls) != 0 {
			t.Errorf("brew calls = %v, want none", r.calls)
		}
	})

	t.Run("re-activation swaps an existing link", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"one", "two"} {
			if _, err := Create(dir, name, ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := activateForTest(t, dir, "one"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := activateForTest(t, dir, "two"); err != nil {
			t.Fatalf("second Activate() error: %v", err)
		}
		name, err := ActiveName(dir)
		if err != nil || name != "two" {
			t.Errorf("ActiveName() = %q, %v; want two", name, err)
		}
	})

	t.Run("unknown profile is ErrNotFound", func(t *testing.T) {
		_, _, err := activateForTest(t, t.TempDir(), "ghost")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestActiveDir(t *testing.T) {
	t.Run("no symlink is ErrNoActiveProfile", func(t *testing.T) {
		_, err := ActiveDir(t.TempDir())
		if !errors.Is(err, ErrNoActiveProfile) {
			t.Errorf("error = %v, want ErrNoActiveProfile", err)
		}
	})

	t.Run("absolute symlink targets pass through", func(t *testing.T) {
		dir := t.TempDir()
		abs := filepath.Join(dir, "elsewhere")
		if err := os.Mkdir(abs, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(abs, filepath.Join(dir, "active-profile")); err != nil {
			t.Fatal(err)
		}
		got, err := ActiveDir(dir)
		if err != nil || got != abs {
			t.Errorf("ActiveDir() = %q, %v; want %q", got, err, abs)
		}
	})
}
