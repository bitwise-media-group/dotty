// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// linkProfile mirrors how dotty exposes a repository profile on a machine:
// the content lives in repo/profiles/<name> and the config dir carries a
// symlink to it.
func linkProfile(t *testing.T, configDir, repo, name string) (site, backing string) {
	t.Helper()
	backing = filepath.Join(repo, "profiles", name)
	if err := os.MkdirAll(backing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backing, metadataFile),
		[]byte(`{"name":"`+name+`"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	site = Dir(configDir, name)
	if err := os.Symlink(backing, site); err != nil {
		t.Fatal(err)
	}
	return site, backing
}

func TestLocate(t *testing.T) {
	t.Run("real directory has no backing", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, "work", ""); err != nil {
			t.Fatal(err)
		}
		site, backing, err := Locate(dir, "work")
		if err != nil {
			t.Fatalf("Locate() error: %v", err)
		}
		if site != Dir(dir, "work") || backing != "" {
			t.Errorf("Locate() = %q, %q; want %q, \"\"", site, backing, Dir(dir, "work"))
		}
	})

	t.Run("symlink resolves to the repository directory", func(t *testing.T) {
		dir, repo := t.TempDir(), t.TempDir()
		wantSite, wantBacking := linkProfile(t, dir, repo, "work")
		site, backing, err := Locate(dir, "work")
		if err != nil {
			t.Fatalf("Locate() error: %v", err)
		}
		if site != wantSite || backing != wantBacking {
			t.Errorf("Locate() = %q, %q; want %q, %q", site, backing, wantSite, wantBacking)
		}
	})

	t.Run("relative symlink resolves against the config dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "elsewhere"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("elsewhere", Dir(dir, "work")); err != nil {
			t.Fatal(err)
		}
		_, backing, err := Locate(dir, "work")
		if err != nil {
			t.Fatalf("Locate() error: %v", err)
		}
		if want := filepath.Join(dir, "elsewhere"); backing != want {
			t.Errorf("backing = %q, want %q", backing, want)
		}
	})

	t.Run("missing profile is ErrNotFound", func(t *testing.T) {
		if _, _, err := Locate(t.TempDir(), "ghost"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("removes the link and the repository directory behind it", func(t *testing.T) {
		dir, repo := t.TempDir(), t.TempDir()
		site, backing := linkProfile(t, dir, repo, "work")
		if _, err := Create(dir, "personal", ""); err != nil {
			t.Fatal(err)
		}
		if _, _, err := activateForTest(t, dir, "personal"); err != nil {
			t.Fatal(err)
		}

		if err := Delete(dir, "work"); err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		if _, err := os.Lstat(site); !os.IsNotExist(err) {
			t.Errorf("lstat %s = %v, want not-exist", site, err)
		}
		if _, err := os.Stat(backing); !os.IsNotExist(err) {
			t.Errorf("stat %s = %v, want not-exist", backing, err)
		}
		assertActive(t, dir, "personal")
	})

	t.Run("removes a profile that is a real directory", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"work", "personal"} {
			if _, err := Create(dir, name, ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := Delete(dir, "work"); err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		if Exists(dir, "work") {
			t.Error("Exists(work) = true after Delete")
		}
		if !Exists(dir, "personal") {
			t.Error("Delete removed the wrong profile")
		}
	})

}

func TestDeleteRefuses(t *testing.T) {
	t.Run("the last profile cannot be deleted", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Create(dir, "work", ""); err != nil {
			t.Fatal(err)
		}
		if err := Delete(dir, "work"); !errors.Is(err, ErrLastProfile) {
			t.Errorf("error = %v, want ErrLastProfile", err)
		}
		if !Exists(dir, "work") {
			t.Error("a refused delete still removed the profile")
		}
	})

	t.Run("the active profile cannot be deleted", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"work", "personal"} {
			if _, err := Create(dir, name, ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := activateForTest(t, dir, "work"); err != nil {
			t.Fatal(err)
		}
		if err := Delete(dir, "work"); !errors.Is(err, ErrActiveProfile) {
			t.Errorf("error = %v, want ErrActiveProfile", err)
		}
		if !Exists(dir, "work") {
			t.Error("a refused delete still removed the profile")
		}
	})

	t.Run("unknown profile is ErrNotFound", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"work", "personal"} {
			if _, err := Create(dir, name, ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := Delete(dir, "ghost"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}
