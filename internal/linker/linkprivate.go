// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
)

// LinkPrivate links the decrypted private tree over $HOME. The tree's source
// runs through the data-dir active-profile symlink, so every created link
// carries that component and `dotty profile activate` swaps the whole
// private identity by retargeting one symlink — the same mechanism LinkHome
// uses for per-profile renders. Conflicts resolve like any other link
// (backup, adopt, skip, fail), sharing the backup scheme dotty dotfiles
// restore reads. An absent decrypted tree links nothing: this machine's
// clone simply carries no files for the active profile.
func LinkPrivate(ios cli.IOStreams, dataDir, home, onConflict string) (Report, string, error) {
	source := filepath.Join(privdot.ActiveLink(dataDir), "home")
	if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
		return Report{}, "", nil
	} else if err != nil {
		return Report{}, "", err
	}

	// The private tree routinely lands entries under ~/.ssh; make sure a
	// fresh machine gets it at 0700 rather than a folded 0755 link chain.
	if err := cli.EnsureDir(filepath.Join(home, ".ssh"), 0o700); err != nil {
		return Report{}, "", err
	}

	backupDir := filepath.Join(dataDir, "backups", time.Now().Format("2006-01-02T15-04-05"))
	if err := cli.EnsureDir(filepath.Join(dataDir, "backups"), 0o700); err != nil {
		return Report{}, "", err
	}
	resolve, err := newResolver(ios, onConflict)
	if err != nil {
		return Report{}, "", err
	}
	report, err := Apply(Tree{Source: source, Target: home}, resolve, backupDir)
	return report, backupDir, err
}
