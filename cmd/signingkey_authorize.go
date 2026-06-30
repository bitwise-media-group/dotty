// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/securitykey"
	"github.com/bitwise-media-group/dotty/internal/signingkey"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// SigningKeyAuthorizeFlags holds the flags for `dotty signing-key authorize`.
type SigningKeyAuthorizeFlags struct {
	Path    string
	Options string
}

var signingKeyAuthorizeFlags = SigningKeyAuthorizeFlags{}

var signingKeyAuthorizeCmd = &cobra.Command{
	Use:   "authorize <[user@]host>",
	Short: "Authorize a signing key for SSH login on a remote host.",
	Long: `Append a signing key's public key to a remote host's authorized_keys so the
YubiKey can log in there over SSH. Only keys on a currently plugged-in YubiKey
are offered; a single match is used directly, otherwise dotty shows a
fuzzy-filterable picker. Narrow the choice up front with --security-key and
--username.

dotty connects with your own ssh client, so ~/.ssh/config, the agent,
known_hosts, and whatever auth the host already accepts all apply — you must be
able to reach the host now to enrol the key for later. The authorized_keys file
is only appended to, never rewritten; ~/.ssh (0700) and the file (0600) are
created if missing. Authorizing a key already on file is an error and changes
nothing.

The entry is prefixed with --options (default no-touch-required): dotty's keys
are enrolled no-touch-required, and sshd rejects a no-touch signature unless the
authorized_keys line says so. Extend it for more control — e.g.
--options=no-touch-required,verify-required to also demand the FIDO PIN, or pass
--options="" to write a bare key.`,
	Example: `  dotty signing-key authorize deavon@server
  dotty signing-key authorize --security-key=work root@host
  dotty signing-key authorize --path=/etc/ssh/keys/authorized_keys admin@host`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		host := args[0]

		if !ios.IsInteractive() {
			return fmt.Errorf(
				"authorize needs an interactive terminal: it prompts for the key to use and ssh prompts to reach %s",
				host)
		}

		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		store, err := securitykey.LoadStore(securitykey.StorePath(dataDir))
		if err != nil {
			return err
		}

		ref, ok, err := selectPluggedKey(cmd.Context(), ios, dataDir, store)
		if err != nil {
			return err
		}
		if !ok {
			return nil // nothing matched, or the user backed out
		}

		_, pub, err := signingkey.Read(ref)
		if err != nil {
			return err
		}

		path := signingKeyAuthorizeFlags.Path
		confirmed, err := tui.Confirm(ios,
			fmt.Sprintf("Authorize %s key for %s on %s?", ref.Type, ref.User, host),
			fmt.Sprintf("Appends the public key to %s; existing keys are kept.", path))
		if errors.Is(err, tui.ErrAborted) || (err == nil && !confirmed) {
			return nil
		}
		if err != nil {
			return err
		}

		opts := signingKeyAuthorizeFlags.Options
		if err := signingkey.Authorize(cmd.Context(), newRunner(ios), host, path, opts, pub); err != nil {
			if errors.Is(err, signingkey.ErrAlreadyAuthorized) {
				return fmt.Errorf("%s key for %s is already authorized on %s", ref.Type, ref.User, host)
			}
			return err
		}
		tui.Successf(ios, "Authorized %s key for %s on %s", ref.Type, ref.User, host)
		return nil
	},
}

// selectPluggedKey resolves the signing key to authorize from the plugged-in
// YubiKeys, honouring --security-key and --username. A single match is used
// directly; several present a fuzzy picker. ok is false when the user backs out
// of the picker.
func selectPluggedKey(
	ctx context.Context, ios cli.IOStreams, dataDir string, store *securitykey.Store,
) (signingkey.KeyRef, bool, error) {
	plugged, err := securitykey.ListSerials(ctx, newRunner(ios))
	if err != nil {
		return signingkey.KeyRef{}, false, err
	}
	serials := plugged
	if ref := signingKeyFlags.SecurityKey; ref != "" {
		want := ref
		if !securitykey.IsSerial(ref) {
			if want, err = store.ResolveName(ref); err != nil {
				return signingkey.KeyRef{}, false, err
			}
		}
		serials = nil
		for _, s := range plugged {
			if s == want {
				serials = []string{s}
			}
		}
	}
	if len(serials) == 0 {
		return signingkey.KeyRef{}, false, fmt.Errorf("no matching YubiKey is plugged in")
	}

	refs, err := signingkey.Scan(dataDir, serials, signingKeyFlags.Username)
	if err != nil {
		return signingkey.KeyRef{}, false, err
	}
	if len(refs) == 0 {
		return signingkey.KeyRef{}, false, fmt.Errorf(
			"no signing keys for the plugged-in YubiKey(s) (run `dotty signing-key new`)")
	}
	if len(refs) == 1 {
		return refs[0], true, nil
	}

	aliases := store.AliasesBySerial()
	headers := []string{"SERIAL", "ALIASES", "TYPE", "USERNAME"}
	rows := make([]tui.TableRow, len(refs))
	for i, ref := range refs {
		var names []string
		for _, a := range aliases[ref.Serial] {
			names = append(names, a.Name)
		}
		rows[i] = tui.TableRow{
			Cells: []string{ref.Serial, strings.Join(names, ", "), ref.Type, ref.User},
			Value: ref.PrivPath,
		}
	}
	value, ok, err := tui.FilterTable(ios, "Which signing key? (enter authorizes, esc cancels)", headers, rows)
	if err != nil || !ok {
		return signingkey.KeyRef{}, false, err
	}
	for _, ref := range refs {
		if ref.PrivPath == value {
			return ref, true, nil
		}
	}
	return signingkey.KeyRef{}, false, nil
}

func init() {
	signingKeyAuthorizeCmd.Flags().StringVar(&signingKeyAuthorizeFlags.Path, "path", "~/.ssh/authorized_keys",
		"remote authorized_keys file to append to")
	signingKeyAuthorizeCmd.Flags().StringVar(&signingKeyAuthorizeFlags.Options, "options", "no-touch-required",
		"authorized_keys option list to prefix (comma-separated; empty for none)")
	signingKeyCmd.AddCommand(signingKeyAuthorizeCmd)
}
