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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple name", input: "work", wantErr: false},
		{name: "with dashes and digits", input: "work-m2", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "reserved symlink name", input: "active-profile", wantErr: true},
		{name: "dot", input: ".", wantErr: true},
		{name: "dotdot", input: "..", wantErr: true},
		{name: "path separator", input: "a/b", wantErr: true},
		{name: "backslash", input: `a\b`, wantErr: true},
		{name: "hidden", input: ".secret", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	t.Run("creates directory and metadata", func(t *testing.T) {
		dir := t.TempDir()
		p, err := Create(dir, "work", "my work machine")
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if p.Name != "work" || p.Description != "my work machine" {
			t.Errorf("profile = %+v", p)
		}
		if p.CreatedAt.IsZero() {
			t.Error("CreatedAt is zero")
		}
		if !Exists(dir, "work") {
			t.Error("Exists() = false after Create")
		}
		loaded, err := Load(dir, "work")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if loaded.Description != "my work machine" {
			t.Errorf("loaded description = %q", loaded.Description)
		}
	})

	t.Run("duplicate name fails with ErrExists", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, "work", ""); err != nil {
			t.Fatal(err)
		}
		_, err := Create(dir, "work", "")
		if !errors.Is(err, ErrExists) {
			t.Errorf("error = %v, want ErrExists", err)
		}
	})

	t.Run("invalid name fails", func(t *testing.T) {
		if _, err := Create(t.TempDir(), "active-profile", ""); err == nil {
			t.Error("Create(active-profile) error = nil")
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("missing profile is ErrNotFound", func(t *testing.T) {
		_, err := Load(t.TempDir(), "ghost")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("directory without metadata is usable", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "copied"), 0o755); err != nil {
			t.Fatal(err)
		}
		p, err := Load(dir, "copied")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if p.Name != "copied" {
			t.Errorf("Name = %q", p.Name)
		}
	})
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zeta", "alpha"} {
		if _, err := Create(dir, name, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := activateForTest(t, dir, "alpha"); err != nil {
		t.Fatal(err)
	}

	profiles, err := List(dir)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2 (active-profile symlink must not count)", len(profiles))
	}
	if profiles[0].Name != "alpha" || profiles[1].Name != "zeta" {
		t.Errorf("order = %s, %s; want alpha, zeta", profiles[0].Name, profiles[1].Name)
	}

	t.Run("missing config dir lists empty", func(t *testing.T) {
		profiles, err := List(filepath.Join(t.TempDir(), "missing"))
		if err != nil || profiles != nil {
			t.Errorf("List(missing) = %v, %v; want nil, nil", profiles, err)
		}
	})
}
