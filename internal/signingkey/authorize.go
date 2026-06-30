// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// ErrAlreadyAuthorized reports that the host's authorized_keys already lists the
// public key, so nothing was appended.
var ErrAlreadyAuthorized = errors.New("public key already in authorized_keys")

// authDuplicateExit is the exit code the remote script uses to signal that the
// key is already present — distinct from ssh's own failures (255) so the two
// never blur together.
const authDuplicateExit = 3

// Authorize appends pub to the authorized_keys file at remotePath on host (an
// ssh destination such as "user@host"), creating the parent directory (0700)
// and the file (0600) when missing. The whole read-check-append runs as one
// remote command so there is no time-of-check/time-of-use gap, and ssh's own
// auth prompts (password, host-key, touch) reach the terminal.
//
// options is a comma-separated authorized_keys option list prefixed to the
// entry (empty for none). dotty enrols keys as no-touch-required, and sshd
// rejects a no-touch signature unless the entry carries the matching
// no-touch-required option — so the default belongs here, not in the key.
//
// It returns ErrAlreadyAuthorized, leaving the file untouched, when an entry
// with the same key identity (algorithm and blob, per PubKeyID) is already
// present, whatever options or comment that entry carries.
func Authorize(ctx context.Context, r interactiveRunner, host, remotePath, options string, pub []byte) error {
	script := buildAuthorizeScript(remotePath, options, pub)
	err := r.RunInteractive(ctx, "ssh", host, script)
	if err == nil {
		return nil
	}
	var exit *cli.ExitError
	if errors.As(err, &exit) && exit.Code == authDuplicateExit {
		return ErrAlreadyAuthorized
	}
	return fmt.Errorf("update authorized_keys on %s: %w", host, err)
}

// buildAuthorizeScript renders the POSIX-sh program run on the remote host. It
// matches a key by its identity (PubKeyID — algorithm and blob, ignoring any
// options prefix or free-form comment) so a re-comment or option change never
// counts as a new key, and exits authDuplicateExit when that identity is
// already on file. The grep needle is the bare algorithm+blob, found as a
// substring, so an entry written without options still registers as present.
func buildAuthorizeScript(remotePath, options string, pub []byte) string {
	id := PubKeyID(string(pub))
	entry := strings.TrimSpace(firstLine(string(pub)))
	if options = strings.TrimSpace(options); options != "" {
		entry = options + " " + entry
	}
	return strings.Join([]string{
		"set -eu",
		"f=" + remoteFileExpr(remotePath),
		`d=$(dirname "$f")`,
		`mkdir -p "$d"`,
		`chmod 700 "$d"`,
		`touch "$f"`,
		`chmod 600 "$f"`,
		`if grep -qF ` + shellSingleQuote(id) + ` "$f"; then exit ` + fmt.Sprint(authDuplicateExit) + `; fi`,
		`printf '%s\n' ` + shellSingleQuote(entry) + ` >> "$f"`,
	}, "\n")
}

// remoteFileExpr renders path as a double-quoted shell word, rewriting a leading
// ~/ to $HOME/ (a tilde does not expand inside quotes) while escaping the
// caller-supplied remainder so it cannot inject further shell expansion.
func remoteFileExpr(path string) string {
	switch {
	case path == "~":
		return `"$HOME"`
	case strings.HasPrefix(path, "~/"):
		return `"$HOME/` + shellDoubleQuoteBody(path[2:]) + `"`
	default:
		return `"` + shellDoubleQuoteBody(path) + `"`
	}
}

// shellSingleQuote wraps s in single quotes for literal use in a shell command.
// An embedded single quote is closed, escaped with a backslash, and reopened —
// the standard POSIX-sh idiom — so no input can break out of the quoting.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellDoubleQuoteBody escapes s for placement inside an existing pair of double
// quotes, neutralizing the characters the shell still interprets there.
func shellDoubleQuoteBody(s string) string {
	return strings.NewReplacer(`\`, `\\`, "`", "\\`", `$`, `\$`, `"`, `\"`).Replace(s)
}
