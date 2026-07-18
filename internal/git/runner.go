// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import "context"

// Runner runs git (and optionally other tools). Output captures stdout for
// queries; Run streams to the terminal; RunInteractive attaches the full
// terminal so rebases show progress and the signing program can prompt, and
// so a non-zero exit surfaces as *cli.ExitError.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, name string, args ...string) error
	RunInteractive(ctx context.Context, name string, args ...string) error
}
