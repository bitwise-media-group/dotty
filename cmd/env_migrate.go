// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/envmigrate"
	"github.com/bitwise-media-group/dotty/internal/fnox"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// EnvMigrateFlags holds the flags for `dotty env migrate`.
type EnvMigrateFlags struct {
	Namespaces   []string
	SkipKeychain bool
	Purge        bool
	Force        bool
	DryRun       bool
}

var envMigrateFlags = EnvMigrateFlags{}

var envMigrateCmd = &cobra.Command{
	Use:   "migrate [PATH...]",
	Short: "Move dotty env credentials and templates into fnox.",
	Long: `Set up fnox for dotty's use, then carry the legacy store into it. Setup is
idempotent: a software age identity is generated and parked in the macOS
Keychain, and the global config (~/.config/fnox/config.toml) gets an age
provider encrypting to it plus every security key enrolled for the private
dotfiles, so a backup key can decrypt on a machine without the identity.
Secrets only enter fnox exec subprocesses (env = "exec"), never the shell.

Every keychain namespace becomes a fnox profile in the global config (the
"default" namespace lands in the top-level secrets); --namespace limits the
set. Each PATH is a .env.dotty template (default: the one in the working
directory, when present) that becomes a fnox.toml beside it — literals as
defaults, {{ dotty://ns/KEY }} references as encrypted values. Keys fnox
already has are skipped unless --force, so re-running is safe. Failures are
reported per entry and never abort the run. The keychain items and templates
are left in place; --purge deletes a namespace's keychain item once every
credential in it migrated, after confirmation. Delete a template yourself
once fnox exec works in its directory.`,
	Example: `  dotty env migrate
  dotty env migrate --namespace aws --namespace ci --purge
  dotty env migrate --skip-keychain ./svc/.env.dotty
  dotty env migrate --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		files, err := envMigrateFiles(args)
		if err != nil {
			return err
		}
		ios := cli.System()
		runner := newRunner(ios)
		globalConfig, err := fnox.GlobalConfigPath()
		if err != nil {
			return err
		}
		client := fnox.New(runner, globalConfig)
		if err := client.Ensure(); err != nil {
			return err
		}
		if err := fnox.Bootstrap(cmd.Context(), ios, client, migrateRecoveryRecipients(ios)); err != nil {
			return err
		}

		store := env.NewStore(env.NewKeychain(runner))
		migrator := envmigrate.New(ios, store, client)
		opts := envmigrate.Options{
			Namespaces: envMigrateFlags.Namespaces,
			Files:      files,
			Force:      envMigrateFlags.Force,
			Purge:      envMigrateFlags.Purge,
			DryRun:     envMigrateFlags.DryRun,
			Provider:   fnox.AgeProvider,
		}

		var report envmigrate.Report
		if !envMigrateFlags.SkipKeychain {
			r, err := migrateNamespaces(cmd.Context(), ios, store, migrator, opts)
			if err != nil {
				return err
			}
			report.Add(r)
		}
		r, err := migrator.Files(cmd.Context(), opts)
		report.Add(r)
		if err != nil {
			return err
		}

		verb := "Migrated"
		if opts.DryRun {
			verb = "Would migrate"
		}
		tui.Successf(ios, "%s %d credential%s (%d skipped, %d failed)", verb,
			report.Migrated, plural(report.Migrated, "", "s"), report.Skipped, report.Failed)
		if len(report.Purged) > 0 {
			tui.Successf(ios, "Purged keychain namespace%s %s",
				plural(len(report.Purged), "", "s"), strings.Join(report.Purged, ", "))
		}
		return nil
	},
}

// envMigrateFiles resolves the templates to migrate: the given paths, or the
// working directory's .env.dotty when there is one. Every path must exist —
// checked here, before fnox is consulted, so a typo fails fast.
func envMigrateFiles(args []string) ([]string, error) {
	files := args
	if len(files) == 0 {
		switch _, err := os.Stat(defaultEnvFile); {
		case err == nil:
			files = []string{defaultEnvFile}
		case !errors.Is(err, fs.ErrNotExist):
			return nil, fmt.Errorf("stat %s: %w", defaultEnvFile, err)
		}
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("template %s: %w", f, err)
		}
	}
	return files, nil
}

// migrateNamespaces runs the keychain half, resolving the namespace set up
// front so --purge can name what it is about to delete. A platform with no
// legacy keychain has nothing to migrate and says so.
func migrateNamespaces(
	ctx context.Context, ios cli.IOStreams, store *env.Store, m *envmigrate.Migrator, opts envmigrate.Options,
) (envmigrate.Report, error) {
	if len(opts.Namespaces) == 0 {
		namespaces, err := store.Namespaces(ctx)
		if errors.Is(err, env.ErrUnsupported) {
			tui.Infof(ios, "Skipping the keychain: %v", err)
			return envmigrate.Report{}, nil
		}
		if err != nil {
			return envmigrate.Report{}, err
		}
		opts.Namespaces = namespaces
	}
	if opts.Purge && !opts.DryRun && len(opts.Namespaces) > 0 {
		ok, err := confirmPurge(ios, opts.Namespaces)
		if err != nil {
			return envmigrate.Report{}, err
		}
		if !ok {
			tui.Infof(ios, "Keeping the keychain items; migrating without --purge")
			opts.Purge = false
		}
	}
	return m.Namespaces(ctx, opts)
}

// confirmPurge asks before deleting keychain namespaces. Outside a terminal
// there is no way to ask, so it refuses rather than delete silently.
func confirmPurge(ios cli.IOStreams, namespaces []string) (bool, error) {
	ok, err := tui.Confirm(ios,
		fmt.Sprintf("Delete keychain namespace%s %s once migrated?",
			plural(len(namespaces), "", "s"), strings.Join(namespaces, ", ")),
		"Only namespaces whose every credential migrated are deleted.")
	if errors.Is(err, tui.ErrNotInteractive) {
		return false, errors.New("refusing to purge without confirmation; re-run in a terminal or drop --purge")
	}
	if errors.Is(err, tui.ErrAborted) {
		return false, nil
	}
	return ok, err
}

// migrateRecoveryRecipients gathers the security keys enrolled for the
// active profile's private dotfiles; no private repository means none.
func migrateRecoveryRecipients(ios cli.IOStreams) []string {
	repo, err := resolvePrivateRepo()
	if err != nil {
		tui.Infof(ios, "No private repository for recovery recipients (%v)", err)
		return nil
	}
	profile, err := privateProfileName()
	if err != nil {
		tui.Infof(ios, "No active profile for recovery recipients (%v)", err)
		return nil
	}
	return recoveryRecipients(ios, repo, profile)
}

// recoveryRecipients reads the profile's private-dotfiles recipients so the
// fnox age provider can encrypt to them too. An absent recipients file is
// reported, not fatal: they can be added on a later run.
func recoveryRecipients(ios cli.IOStreams, repo, profile string) []string {
	path := privdot.RecipientsPath(repo, profile)
	recipients, err := privdot.ReadRecipients(path)
	if errors.Is(err, fs.ErrNotExist) {
		tui.Infof(ios, "No recipients at %s; enroll a security key with dotty private enroll to add recovery", path)
		return nil
	}
	if err != nil {
		tui.Warnf(ios, "%v", err)
		return nil
	}
	return recipients
}

func init() {
	envMigrateCmd.Flags().StringArrayVar(&envMigrateFlags.Namespaces, "namespace", nil,
		"keychain namespace to migrate (repeatable; default: all)")
	envMigrateCmd.Flags().BoolVar(&envMigrateFlags.SkipKeychain, "skip-keychain", false,
		"migrate only the given templates, not the keychain")
	envMigrateCmd.Flags().BoolVar(&envMigrateFlags.Purge, "purge", false,
		"delete each keychain namespace once every credential in it migrated")
	envMigrateCmd.Flags().BoolVar(&envMigrateFlags.Force, "force", false,
		"overwrite keys fnox already has")
	envMigrateCmd.Flags().BoolVar(&envMigrateFlags.DryRun, "dry-run", false,
		"report what would be migrated without writing anything")
	envCmd.AddCommand(envMigrateCmd)
}
