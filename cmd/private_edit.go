// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var privateEditCmd = &cobra.Command{
	Use:   "edit <path>",
	Short: "Edit an encrypted entry in place.",
	Long: `Decrypt one entry to a 0600 scratch file inside the private data area
(never a shared temp directory), open it in $VISUAL/$EDITOR, and encrypt the
result back to the profile's recipients. Unchanged content is left alone —
age output is not deterministic, so re-encrypting an identical file would
only manufacture a spurious diff. A plaintext (unencrypted) entry opens
directly in the repository.`,
	Example: `  dotty private edit .config/private/git/config
  dotty private edit ~/.ssh/config.d/personal.conf`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		ctx := cmd.Context()
		repo, err := resolvePrivateRepo()
		if err != nil {
			return err
		}
		name, err := privateProfileName()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		rel, err := homeRel(args[0], home)
		if err != nil {
			return err
		}
		runner := newRunner(ios)

		ctPath := filepath.Join(privdot.HomeDir(repo, name), rel+privdot.CipherExt)
		if _, err := os.Stat(ctPath); err != nil {
			plainRepo := filepath.Join(privdot.HomeDir(repo, name), rel)
			if _, plainErr := os.Stat(plainRepo); plainErr == nil {
				return cli.EditFile(ctx, runner, plainRepo)
			}
			return fmt.Errorf("profile %s carries no entry %s: %w", name, rel, err)
		}

		identities, err := privdot.Identities(repo, name)
		if err != nil {
			return err
		}
		if err := privdot.RequireTerminal(ios, identities); err != nil {
			return err
		}
		ciphertext, err := os.ReadFile(ctPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", ctPath, err)
		}
		plaintext, err := privdot.Decrypt(ctx, runner, identities, ciphertext)
		if err != nil {
			return err
		}

		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		scratch, err := privateScratchFile(dataDir, rel, plaintext)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(scratch) }()
		if err := cli.EditFile(ctx, runner, scratch); err != nil {
			return err
		}
		edited, err := os.ReadFile(scratch)
		if err != nil {
			return fmt.Errorf("read %s back: %w", scratch, err)
		}
		if bytes.Equal(edited, plaintext) {
			tui.Infof(ios, "%s unchanged", rel)
			return nil
		}
		if err := privdot.Store(ctx, runner, repo, name, dataDir, rel, edited); err != nil {
			return err
		}
		tui.Successf(ios, "Re-encrypted %s; commit the ciphertext in %s", rel, repo)
		return nil
	},
}

// privateScratchFile writes plaintext to a 0600 scratch file inside the
// private data root, where the 0700 boundary already protects it.
func privateScratchFile(dataDir, rel string, plaintext []byte) (string, error) {
	root := privdot.DataRoot(dataDir)
	if err := cli.EnsureDir(root, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(root, "edit-"+filepath.Base(rel)+"-*")
	if err != nil {
		return "", fmt.Errorf("create scratch file: %w", err)
	}
	path := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("chmod %s: %w", path, err)
	}
	if _, err := f.Write(plaintext); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close %s: %w", path, err)
	}
	return path, nil
}

func init() {
	privateCmd.AddCommand(privateEditCmd)
}
