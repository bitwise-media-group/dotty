// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/securitykey"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PrivateEnrollFlags holds the flags for `dotty private enroll`.
type PrivateEnrollFlags struct {
	Serial      string
	SecurityKey string
	Slot        int
	PINPolicy   string
	TouchPolicy string
}

var privateEnrollFlags = PrivateEnrollFlags{}

var privateEnrollCmd = &cobra.Command{
	Use:   "enroll",
	Short: "Add a security key to the profile's recipients.",
	Long: `Create an age identity on a YubiKey (a PIV retired slot, via
age-plugin-yubikey), commit its non-secret identity stub beside the
profile's recipients, and add the recipient — from then on every encrypt
targets the key, and any machine with it plugged in decrypts. The PIV
standard slots (smart-card login, signing) are never touched, but the PIV
PIN and its retry counter are shared with them: three wrong PINs at decrypt
time lock PIV login too.

The default policies cost one PIN per PIV session and one touch per
15-second window (pin-policy once, touch-policy cached), so a whole-tree
decrypt is a single PIN+touch burst. Enrolling a second key onto a profile
that already has ciphertext prompts a rekey — existing files never learn
about a new recipient on their own.`,
	Example: `  dotty private enroll
  dotty private enroll --serial 17741369
  dotty private enroll --security-key=backup --touch-policy=always`,
	Args: cobra.NoArgs,
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
		serial, err := resolveEnrollSerial(ctx, ios)
		if err != nil {
			return err
		}
		if err := requireAllowedSerial(serial); err != nil {
			return err
		}

		tui.Infof(ios, "The plugin will ask for the PIV management key or touch; watch the terminal")
		recipient, slot, err := privdot.Enroll(ctx, newRunner(ios), repo, name, privdot.EnrollOptions{
			Serial:      serial,
			Slot:        privateEnrollFlags.Slot,
			PINPolicy:   privateEnrollFlags.PINPolicy,
			TouchPolicy: privateEnrollFlags.TouchPolicy,
		})
		if err != nil {
			return err
		}
		tui.Successf(ios, "Enrolled YubiKey %s (slot %d) for profile %s", serial, slot, name)
		tui.Infof(ios, "Recipient: %s", recipient)

		if hasCiphertext(repo, name) {
			tui.Warnf(ios, "Profile %s already has encrypted files; run dotty private rekey so this key can open them", name)
		}
		tui.Infof(ios, "Commit the identity stub and recipients in %s", repo)
		return nil
	},
}

// resolveEnrollSerial picks the YubiKey to enroll: --serial, --security-key
// (alias or serial), or the single allowed plugged-in key — with a picker
// when several qualify.
func resolveEnrollSerial(ctx context.Context, ios cli.IOStreams) (string, error) {
	if privateEnrollFlags.Serial != "" {
		if !securitykey.IsSerial(privateEnrollFlags.Serial) {
			return "", fmt.Errorf("--serial: %q is not a serial number", privateEnrollFlags.Serial)
		}
		return privateEnrollFlags.Serial, nil
	}
	store, err := keyStore()
	if err != nil {
		return "", err
	}
	if ref := privateEnrollFlags.SecurityKey; ref != "" {
		if securitykey.IsSerial(ref) {
			return ref, nil
		}
		return store.ResolveName(ref)
	}

	plugged, err := securitykey.ListSerials(ctx, newRunner(ios))
	if err != nil {
		return "", fmt.Errorf("list connected security keys (or pass --serial): %w", err)
	}
	if plugged, err = filterAllowedSerials(plugged); err != nil {
		return "", err
	}
	switch len(plugged) {
	case 0:
		return "", errors.New("no allowed security key connected; plug one in or pass --serial")
	case 1:
		return plugged[0], nil
	}
	options := make([]tui.Option, len(plugged))
	for i, serial := range plugged {
		options[i] = tui.Option{Label: securitykey.SerialLabel(store, serial), Value: serial}
	}
	picked, err := tui.FuzzySelect(ios, "Enroll which security key?", options)
	if errors.Is(err, tui.ErrNotInteractive) {
		return "", errors.New("several keys connected; pass --serial or --security-key")
	}
	return picked, err
}

// hasCiphertext reports whether the profile already carries encrypted
// entries — the signal a fresh recipient needs a rekey to read them.
func hasCiphertext(repo, profile string) bool {
	files, err := privdot.ListFiles(repo, profile)
	if err != nil {
		return false
	}
	for _, f := range files {
		if f.Encrypted {
			return true
		}
	}
	return false
}

func init() {
	privateEnrollCmd.Flags().StringVar(&privateEnrollFlags.Serial, "serial", "", "YubiKey serial to enroll")
	privateEnrollCmd.Flags().StringVar(&privateEnrollFlags.SecurityKey, "security-key", "",
		"security-key alias (or serial) to enroll")
	privateEnrollCmd.Flags().IntVar(&privateEnrollFlags.Slot, "slot", 0,
		fmt.Sprintf("identity slot 1-%d (default: first free)", privdot.PluginSlots))
	privateEnrollCmd.Flags().StringVar(&privateEnrollFlags.PINPolicy, "pin-policy", "once",
		"PIV PIN policy: always, once, or never")
	privateEnrollCmd.Flags().StringVar(&privateEnrollFlags.TouchPolicy, "touch-policy", "cached",
		"touch policy: always, cached (15s), or never")
	privateCmd.AddCommand(privateEnrollCmd)
}
