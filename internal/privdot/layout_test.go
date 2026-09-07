// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// testProfile is the profile every seeding helper targets; tests needing a
// second profile scaffold it explicitly.
const testProfile = "personal"

// seedRepo builds a minimal private repository with the test profile
// carrying the given home-relative entries (paths ending in .age are
// ciphertext).
func seedRepo(t *testing.T, entries map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	if err := Scaffold(repo, testProfile); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	for rel, content := range entries {
		path := filepath.Join(HomeDir(repo, testProfile), rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestListFiles(t *testing.T) {
	repo := seedRepo(t, map[string]string{
		".ssh/known_hosts.age":           "ct",
		".config/private/git/config.age": "ct",
		".ssh/config.d/base.conf":        "plain",
	})
	files, err := ListFiles(repo, "personal")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	rels := make([]string, 0, len(files))
	encrypted := map[string]bool{}
	for _, f := range files {
		rels = append(rels, f.Rel)
		encrypted[f.Rel] = f.Encrypted
	}
	want := []string{".config/private/git/config", ".ssh/config.d/base.conf", ".ssh/known_hosts"}
	if !reflect.DeepEqual(rels, want) {
		t.Errorf("rels = %v, want %v (sorted, .age stripped, .gitkeep skipped)", rels, want)
	}
	if !encrypted[".ssh/known_hosts"] || encrypted[".ssh/config.d/base.conf"] {
		t.Errorf("encrypted flags wrong: %v", encrypted)
	}
}

func TestListFilesMissingProfile(t *testing.T) {
	repo := seedRepo(t, nil)
	files, err := ListFiles(repo, "work")
	if err != nil || files != nil {
		t.Errorf("missing profile should be empty, got %v, %v", files, err)
	}
}

func TestIdentities(t *testing.T) {
	repo := seedRepo(t, nil)
	for _, serial := range []string{"23899167", "17741369"} {
		if err := os.WriteFile(IdentityPath(repo, "personal", serial), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := Identities(repo, "personal")
	if err != nil {
		t.Fatalf("Identities: %v", err)
	}
	want := []string{
		IdentityPath(repo, "personal", "17741369"),
		IdentityPath(repo, "personal", "23899167"),
	}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("Identities = %v, want %v", ids, want)
	}
}

func TestScaffoldIdempotentAndMarked(t *testing.T) {
	repo := t.TempDir()
	if err := Scaffold(repo, "personal"); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if !IsRepo(repo) {
		t.Error("scaffolded repository should carry the private marker")
	}
	hook := filepath.Join(repo, ".githooks", "pre-commit")
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatalf("pre-commit hook missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("pre-commit hook is not executable")
	}
	// A second run must not clobber user edits.
	custom := []byte("# custom rules\n")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Scaffold(repo, "work"); err != nil {
		t.Fatalf("Scaffold rerun: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil || string(got) != string(custom) {
		t.Errorf("rerun clobbered .gitignore: %q, %v", got, err)
	}
	if _, err := os.Stat(HomeDir(repo, "work")); err != nil {
		t.Errorf("rerun should add the new profile: %v", err)
	}
}

func TestScaffoldSeedsSkeletonDirs(t *testing.T) {
	repo := t.TempDir()
	if err := Scaffold(repo, "personal"); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	for _, rel := range skeletonDirs {
		keep := filepath.Join(HomeDir(repo, "personal"), rel, gitkeepName)
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("skeleton dir %s missing its placeholder: %v", rel, err)
		}
	}
	// The skeleton itself keeps home committable; no placeholder needed there.
	if _, err := os.Stat(filepath.Join(HomeDir(repo, "personal"), gitkeepName)); err == nil {
		t.Error("home tree should not carry a stray placeholder beside the skeleton")
	}
	// ListFiles must still see the profile as empty — the skeleton is
	// repository furniture, never deployed.
	files, err := ListFiles(repo, "personal")
	if err != nil || files != nil {
		t.Errorf("skeleton should deploy nothing, got %v, %v", files, err)
	}
}

func TestScaffoldAdoptsPopulatedDirsQuietly(t *testing.T) {
	repo := t.TempDir()
	conf := filepath.Join(HomeDir(repo, "personal"), ".ssh", "config.d", "personal.conf.age")
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, []byte("ct"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Scaffold(repo, "personal"); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if _, err := os.Stat(conf); err != nil {
		t.Errorf("adopted file must survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(conf), gitkeepName)); err == nil {
		t.Error("populated skeleton dir should not gain a placeholder")
	}
}

func TestAdoptableRoot(t *testing.T) {
	// Symlinks make path equality slippery (macOS tmp lives under
	// /var -> /private/var, and Getwd may return either form), so compare
	// resolved paths on both sides.
	realDir := func(dir string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	tests := []struct {
		name    string
		seed    func(t *testing.T, root string) // shape the repo
		fromSub bool                            // run from root/sub instead of root
		want    bool                            // root expected back
	}{
		{"marked repository from a subdirectory", func(t *testing.T, root string) {
			if err := Scaffold(root, "personal"); err != nil {
				t.Fatal(err)
			}
		}, true, true},
		{"fresh clone with hosting furniture", func(t *testing.T, root string) {
			for _, name := range []string{"README.md", "LICENSE", ".gitignore"} {
				if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}, false, true},
		{"repository with unrelated content", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			tt.seed(t, root)
			cwd := root
			if tt.fromSub {
				cwd = filepath.Join(root, "sub")
				if err := os.MkdirAll(cwd, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(cwd)
			got := AdoptableRoot()
			switch {
			case tt.want && (got == "" || realDir(got) != realDir(root)):
				t.Errorf("AdoptableRoot() = %q, want %q", got, root)
			case !tt.want && got != "":
				t.Errorf("AdoptableRoot() = %q, want \"\"", got)
			}
		})
	}
	t.Run("outside any repository", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if got := AdoptableRoot(); got != "" {
			t.Errorf("AdoptableRoot() = %q, want \"\"", got)
		}
	})
}

func TestListProfiles(t *testing.T) {
	repo := t.TempDir()
	if err := Scaffold(repo, "personal"); err != nil {
		t.Fatal(err)
	}
	if err := Scaffold(repo, "work"); err != nil {
		t.Fatal(err)
	}
	names, err := ListProfiles(repo)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"personal", "work"}) {
		t.Errorf("ListProfiles = %v", names)
	}
}
