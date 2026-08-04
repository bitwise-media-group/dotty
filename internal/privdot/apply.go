// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// ApplyReport summarizes one Materialize: how many entries were decrypted or
// copied fresh, how many were already current, which were skipped (no key,
// local drift, conflicts), and which manifest entries were pruned because
// the profile no longer carries them — their live sites may now dangle.
type ApplyReport struct {
	Decrypted int
	Copied    int
	UpToDate  int
	Skipped   []string
	Pruned    []string
}

// Materialize lands a profile's plaintext tree under the data directory,
// incrementally: an entry is re-decrypted only when its ciphertext changed
// since the manifest's record — every decrypt can cost a PIN or touch, so a
// steady-state run touches no hardware at all. Local edits are never
// overwritten: a drifted copy is left for `dotty private encrypt`, and a
// conflict (both sides changed) is skipped with a warning, or fails the run
// when strict. An entry that cannot be decrypted (key unplugged, wrong key)
// is skipped the same way, keeping any previously decrypted copy usable.
func Materialize(ctx context.Context, ios cli.IOStreams, r Runner,
	repo, profile, dataDir string, strict bool) (ApplyReport, error) {
	var rep ApplyReport
	files, err := ListFiles(repo, profile)
	if err != nil {
		return rep, err
	}
	manifestPath := ManifestPath(dataDir, profile)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return rep, err
	}
	identities, err := Identities(repo, profile)
	if err != nil {
		return rep, err
	}

	seen := make(map[string]bool, len(files))
	for _, f := range files {
		seen[f.Rel] = true
		if err := materializeOne(ctx, ios, r, f, profile, dataDir, identities, manifest, &rep, strict); err != nil {
			return rep, err
		}
	}

	// Entries the profile no longer carries: drop the plaintext and the
	// record; the caller prunes the dangling live sites.
	for rel := range manifest {
		if seen[rel] {
			continue
		}
		if err := os.Remove(PlainPath(dataDir, profile, rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return rep, fmt.Errorf("prune %s: %w", rel, err)
		}
		delete(manifest, rel)
		rep.Pruned = append(rep.Pruned, rel)
	}

	if err := manifest.Save(manifestPath); err != nil {
		return rep, err
	}
	return rep, nil
}

// materializeOne lands one entry, updating manifest in place.
func materializeOne(ctx context.Context, ios cli.IOStreams, r Runner, f File,
	profile, dataDir string, identities []string, manifest Manifest, rep *ApplyReport, strict bool) error {
	repoContent, err := os.ReadFile(f.RepoPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", f.RepoPath, err)
	}
	repoHash := HashBytes(repoContent)
	entry, known := manifest[f.Rel]

	plainHash, err := HashFile(PlainPath(dataDir, profile, f.Rel))
	plainExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if known && plainExists && entry.CipherHash == repoHash {
		rep.UpToDate++
		return nil
	}
	if known && plainExists && plainHash != entry.PlainHash {
		// Both sides moved: never clobber the local edit.
		msg := fmt.Sprintf("%s: local edits and repository changes conflict; reconcile, then dotty private encrypt", f.Rel)
		if strict {
			return errors.New(msg)
		}
		tui.Warnf(ios, "%s", msg)
		rep.Skipped = append(rep.Skipped, f.Rel)
		return nil
	}

	plaintext := repoContent
	if f.Encrypted {
		if err := RequireTerminal(ios, identities); err != nil {
			if strict {
				return fmt.Errorf("%s: %w", f.Rel, err)
			}
			tui.Warnf(ios, "%s: %v", f.Rel, err)
			rep.Skipped = append(rep.Skipped, f.Rel)
			return nil
		}
		if plaintext, err = Decrypt(ctx, r, identities, repoContent); err != nil {
			if strict {
				return fmt.Errorf("%s: %w", f.Rel, err)
			}
			tui.Warnf(ios, "%s: %v (kept the previous copy; is the security key plugged in?)", f.Rel, err)
			rep.Skipped = append(rep.Skipped, f.Rel)
			return nil
		}
	}
	if err := WritePlain(dataDir, profile, f.Rel, plaintext); err != nil {
		return err
	}
	manifest[f.Rel] = Entry{CipherHash: repoHash, PlainHash: HashBytes(plaintext), DecryptedAt: time.Now().UTC()}
	if f.Encrypted {
		rep.Decrypted++
	} else {
		rep.Copied++
	}
	return nil
}

// Activate points the data-dir active-profile symlink at profile, creating
// the private root on first use. Like the config-dir counterpart, the link
// is relative, and retargeting it swaps every routed $HOME link at once.
func Activate(dataDir, profile string) error {
	if err := cli.EnsureDir(DataRoot(dataDir), 0o700); err != nil {
		return err
	}
	if err := cli.EnsureDir(filepath.Join(DataRoot(dataDir), profile), 0o700); err != nil {
		return err
	}
	link := ActiveLink(dataDir)
	if existing, err := os.Readlink(link); err == nil {
		if existing == profile {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("retarget %s: %w", link, err)
		}
	}
	if err := os.Symlink(profile, link); err != nil {
		return fmt.Errorf("activate private profile %s: %w", profile, err)
	}
	return nil
}

// StaleRels returns the $HOME-relative paths that other private profiles
// carry but the active one does not — the sites whose links now dangle
// after a profile switch, for the caller to prune.
func StaleRels(repo, active string) ([]string, error) {
	activeFiles, err := ListFiles(repo, active)
	if err != nil {
		return nil, err
	}
	current := make(map[string]bool, len(activeFiles))
	for _, f := range activeFiles {
		current[f.Rel] = true
	}
	profiles, err := ListProfiles(repo)
	if err != nil {
		return nil, err
	}
	var stale []string
	seen := map[string]bool{}
	for _, p := range profiles {
		if p == active {
			continue
		}
		files, err := ListFiles(repo, p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !current[f.Rel] && !seen[f.Rel] {
				seen[f.Rel] = true
				stale = append(stale, f.Rel)
			}
		}
	}
	return stale, nil
}
