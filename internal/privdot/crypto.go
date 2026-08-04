// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// ageBin is the age CLI the crypto layer drives. Recipient and identity
// files are opaque to dotty, so any age plugin on PATH works; dotty itself
// links no crypto.
const ageBin = "age"

// Runner is the slice of ExecRunner the crypto layer consumes; tests
// substitute a fake asserting the argv, and integration tests run the real
// age with software identities.
type Runner interface {
	OutputStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

// ErrNeedsTerminal reports a decryption that requires a hardware plugin
// prompt (PIN, touch) on a run with no terminal to host it. Failing fast
// beats letting the plugin hang looking for a tty.
var ErrNeedsTerminal = errors.New("decryption needs a terminal for the security-key PIN; rerun interactively")

// Encrypt encrypts plaintext to every recipient in recipientsPath and
// returns the ciphertext. The plaintext passes through the pipe only —
// recipient-side age needs no hardware and no prompts.
func Encrypt(ctx context.Context, r Runner, recipientsPath string, plaintext []byte) ([]byte, error) {
	if _, err := r.LookPath(ageBin); err != nil {
		return nil, err
	}
	ct, err := r.OutputStdin(ctx, plaintext, ageBin, "--encrypt", "-R", recipientsPath, "-a")
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return ct, nil
}

// Decrypt decrypts ciphertext with the first of identities that can open it
// and returns the plaintext. age tries each identity in turn, so passing
// every enrolled stub lets whichever key is plugged in win. Hardware-backed
// identities prompt on the process tty (age and its plugins open /dev/tty
// directly); call RequireTerminal first on paths that may run headless.
func Decrypt(ctx context.Context, r Runner, identities []string, ciphertext []byte) ([]byte, error) {
	if len(identities) == 0 {
		return nil, errors.New("no age identities; run dotty private enroll first")
	}
	if _, err := r.LookPath(ageBin); err != nil {
		return nil, err
	}
	args := []string{"--decrypt"}
	for _, id := range identities {
		args = append(args, "-i", id)
	}
	pt, err := r.OutputStdin(ctx, ciphertext, ageBin, args...)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return pt, nil
}

// RequireTerminal fails with ErrNeedsTerminal when any of the identities is
// plugin-backed and the streams cannot host a prompt. Software identities
// (age-keygen) decrypt silently and pass regardless.
func RequireTerminal(ios cli.IOStreams, identities []string) error {
	if ios.IsInteractive() {
		return nil
	}
	for _, id := range identities {
		data, err := os.ReadFile(id)
		if err != nil {
			return fmt.Errorf("read identity %s: %w", id, err)
		}
		if strings.Contains(string(data), "AGE-PLUGIN-") {
			return fmt.Errorf("%s: %w", id, ErrNeedsTerminal)
		}
	}
	return nil
}
