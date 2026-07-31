// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Restore copies every file under backupRoot back to the absolute path it
// mirrors (backupRoot/Users/x/.config/foo restores /Users/x/.config/foo),
// replacing whatever occupies the site — usually the symlink whose creation
// displaced the file. The backup set is copied, not moved, so a restore can
// be repeated and the set only disappears when the user deletes it.
func Restore(backupRoot string) ([]string, error) {
	var restored []string
	err := filepath.WalkDir(backupRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(backupRoot, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		site := string(filepath.Separator) + rel
		if err := os.RemoveAll(site); err != nil {
			return fmt.Errorf("clear %s: %w", site, err)
		}
		if d.Type()&os.ModeSymlink != 0 { // a symlink inside a backed-up directory
			dest, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read link %s: %w", path, err)
			}
			if err := os.Symlink(dest, site); err != nil {
				return fmt.Errorf("relink %s: %w", site, err)
			}
		} else if err := copyFile(path, site); err != nil {
			return err
		}
		restored = append(restored, site)
		return nil
	})
	if err != nil {
		return restored, err
	}
	if len(restored) == 0 {
		return nil, fmt.Errorf("no files under %s", backupRoot)
	}
	return restored, nil
}

// copyFile copies src to dst preserving src's permission bits, creating dst's
// parents.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", src, err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	// OpenFile applies perm only on creation; an existing dst keeps its old
	// mode through the truncate, so enforce src's bits explicitly.
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod %s: %w", dst, err)
	}
	return nil
}
