// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var privateEncryptCmd = &cobra.Command{
	Use:   "encrypt <path>",
	Short: "Adopt a live file into the private profile, encrypted.",
	Long: `Encrypt a file from the home directory to the profile's recipients and
store the ciphertext in the private repository at the matching home-relative
path. The plaintext also lands in the decrypted private area so the entry is
immediately linkable, and the manifest records both hashes. Re-encrypting an
already-adopted entry is how a local edit (status: drifted) gets committed
back.`,
	Example: `  dotty private encrypt ~/.ssh/known_hosts
  dotty private encrypt .config/private/git/config`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
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
		if _, err := os.Stat(privdot.RecipientsPath(repo, name)); err != nil {
			return fmt.Errorf("profile %s has no recipients; run dotty private enroll first: %w", name, err)
		}
		live := filepath.Join(home, rel)
		plaintext, err := os.ReadFile(live)
		if err != nil {
			return fmt.Errorf("read %s: %w", live, err)
		}
		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		runner := newRunner(ios)
		if err := privdot.Store(cmd.Context(), runner, repo, name, dataDir, rel, plaintext); err != nil {
			return err
		}
		tui.Successf(ios, "Encrypted %s into profile %s", rel, name)
		tui.Infof(ios, "Link it with dotty private link; commit the ciphertext in %s", repo)
		return nil
	},
}

func init() {
	privateCmd.AddCommand(privateEncryptCmd)
}
