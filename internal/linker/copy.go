// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// classifyCopy states one copy-deployed directory site: missing sites will be
// created (StateLink), a symlink left by the linked deployment this mode
// replaces or an out-of-sync directory will be resynced (StateRelink), a
// mirror already matching its source is StateOK, and a regular file in the
// way is a conflict.
func classifyCopy(site, dest string) (Action, error) {
	a := Action{Site: site, Dest: dest, Copy: true}
	info, err := os.Lstat(site)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		a.State = StateLink
	case err != nil:
		return a, fmt.Errorf("inspect %s: %w", site, err)
	case info.Mode()&os.ModeSymlink != 0:
		a.State = StateRelink
	case info.IsDir():
		inSync, err := dirInSync(dest, site)
		if err != nil {
			return a, err
		}
		a.State = StateRelink
		if inSync {
			a.State = StateOK
		}
	default:
		a.State = StateConflict
		// Adoption would move the file over the source directory and destroy
		// it; Generated makes resolvers withhold the option.
		a.Generated = true
	}
	return a, nil
}

// applyCopy executes one classified copy Action: the source directory is
// mirrored into the site as real files. A symlink at the site is removed
// first — removing it never destroys the repository directory it names — and
// a conflicting regular file goes through the resolver, where any answer
// short of skip or fail backs it up (adoption is refused, see classifyCopy).
func applyCopy(a Action, resolve Resolver, backupRoot string, rep *Report) error {
	switch a.State {
	case StateOK:
		rep.OK++
		return nil
	case StateRelink:
		if info, err := os.Lstat(a.Site); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(a.Site); err != nil {
				return fmt.Errorf("remove stale symlink %s: %w", a.Site, err)
			}
		}
	case StateConflict:
		res, err := resolve(Conflict{Site: a.Site, Dest: a.Dest, Generated: a.Generated})
		if err != nil {
			return err
		}
		switch res {
		case ResSkip:
			rep.Skipped = append(rep.Skipped, a.Site)
			return nil
		case ResBackup, ResAdopt:
			dst := filepath.Join(backupRoot, strings.TrimPrefix(a.Site, string(filepath.Separator)))
			if err := move(a.Site, dst); err != nil {
				return fmt.Errorf("back up %s: %w", a.Site, err)
			}
			rep.Backed = append(rep.Backed, a.Site)
		default:
			return fmt.Errorf("%s: existing file conflicts with %s", a.Site, a.Dest)
		}
	}
	if err := syncDir(a.Dest, a.Site); err != nil {
		return err
	}
	rep.Copied = append(rep.Copied, a.Site)
	return nil
}

// syncDir mirrors src into dst as real files: entries src carries are copied
// when missing or different, entries src lacks are removed. A copy-deployed
// directory is wholly dotty-managed — it was a folded symlink into the
// repository before copy deployment existed — so pruning extras never removes
// a file that was not repository-managed to begin with.
func syncDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", src, err)
	}
	if err := cli.EnsureDir(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if di, err := os.Lstat(d); err == nil && di.IsDir() != e.IsDir() {
			if err := os.RemoveAll(d); err != nil {
				return fmt.Errorf("clear %s: %w", d, err)
			}
		}
		if e.IsDir() {
			if err := syncDir(s, d); err != nil {
				return err
			}
			continue
		}
		same, err := fileInSync(s, d)
		if err != nil {
			return err
		}
		if !same {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}

	dstEntries, err := os.ReadDir(dst)
	if err != nil {
		return fmt.Errorf("read %s: %w", dst, err)
	}
	for _, e := range dstEntries {
		if names[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("prune %s: %w", filepath.Join(dst, e.Name()), err)
		}
	}
	return nil
}

// dirInSync reports whether dst already mirrors src exactly: the same entry
// names and kinds, and for files the same permission bits and content.
func dirInSync(src, dst string) (bool, error) {
	srcEntries, err := os.ReadDir(src)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", src, err)
	}
	dstEntries, err := os.ReadDir(dst)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dst, err)
	}
	if len(srcEntries) != len(dstEntries) {
		return false, nil
	}
	for _, e := range srcEntries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			di, err := os.Lstat(d)
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			if err != nil {
				return false, fmt.Errorf("inspect %s: %w", d, err)
			}
			if !di.IsDir() {
				return false, nil
			}
			if same, err := dirInSync(s, d); err != nil || !same {
				return same, err
			}
			continue
		}
		if same, err := fileInSync(s, d); err != nil || !same {
			return same, err
		}
	}
	return true, nil
}

// fileInSync reports whether dst exists as a regular file carrying src's
// permission bits and content.
func fileInSync(src, dst string) (bool, error) {
	si, err := os.Stat(src)
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", src, err)
	}
	di, err := os.Lstat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", dst, err)
	}
	if !di.Mode().IsRegular() || di.Mode().Perm() != si.Mode().Perm() || di.Size() != si.Size() {
		return false, nil
	}
	sb, err := os.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", src, err)
	}
	db, err := os.ReadFile(dst)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dst, err)
	}
	return bytes.Equal(sb, db), nil
}
