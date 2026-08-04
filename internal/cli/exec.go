// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// ExecRunner invokes the external programs dotty orchestrates. Area packages
// consume it through their own small interfaces so tests can substitute fakes.
type ExecRunner struct {
	ios IOStreams
	log *slog.Logger
}

// NewExecRunner returns a runner writing streamed output to ios. A nil log is
// replaced with a no-op logger.
func NewExecRunner(ios IOStreams, log *slog.Logger) *ExecRunner {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &ExecRunner{ios: ios, log: log}
}

// Output runs name with args and returns its captured stdout. On a non-zero
// exit the stderr tail is folded into the returned error so callers can wrap
// it without re-capturing.
func (r *ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec output", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("run %s: %w: %s", name, err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.Bytes(), nil
}

// Run runs name with args, streaming stdout and stderr to the IOStreams. Stdin
// is not connected; use RunInteractive for programs that prompt.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec run", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = r.ios.Out
	cmd.Stderr = r.ios.ErrOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

// RunInteractive runs name with args wired to the full IOStreams, stdin
// included. When the streams are the process's own, the child inherits the
// terminal — required for editors, brew prompts, and ssh-keygen PIN entry.
// A non-zero exit comes back as an *ExitError carrying the child's code.
func (r *ExecRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return r.runInteractive(ctx, "", nil, name, args...)
}

// RunInteractiveEnv is RunInteractive with extraEnv appended to the current
// process environment (later entries win, per os/exec semantics), so callers
// can expose secrets to the child without leaking them into dotty's own
// environment. A nil extraEnv leaves the child inheriting dotty's environment
// unchanged. Use RunInteractiveEnvReplace when a variable must be removed or
// reliably overridden — appending can't, since getenv returns the first of any
// duplicate.
func (r *ExecRunner) RunInteractiveEnv(ctx context.Context, extraEnv []string, name string, args ...string) error {
	var env []string
	if len(extraEnv) > 0 {
		env = append(os.Environ(), extraEnv...)
	}
	return r.runInteractive(ctx, "", env, name, args...)
}

// RunInteractiveEnvReplace is RunInteractive with the child's environment set to
// env exactly, replacing dotty's own rather than extending it. The sign path
// uses it to drop SSH_AUTH_SOCK (so ssh-keygen signs with the stub, not the
// agent) and to repoint SSH_ASKPASS at dotty's pinentry bridge — neither of
// which appending can do. env must be complete: pass a slice derived from
// os.Environ() unless a bare environment is intended.
func (r *ExecRunner) RunInteractiveEnvReplace(ctx context.Context, env []string, name string, args ...string) error {
	if env == nil {
		env = []string{} // distinguish "replace with empty" from "inherit"
	}
	return r.runInteractive(ctx, "", env, name, args...)
}

// RunInteractiveDir is RunInteractive with the child's working directory set to
// dir. ssh-keygen -K writes the resident keys it downloads to the current
// directory, so `signing-key import` runs it in a throwaway dir; an empty dir
// leaves the child in dotty's own working directory, as the other variants do.
func (r *ExecRunner) RunInteractiveDir(ctx context.Context, dir, name string, args ...string) error {
	return r.runInteractive(ctx, dir, nil, name, args...)
}

// runInteractive runs name wired to the full IOStreams. A nil env leaves the
// child inheriting dotty's environment; a non-nil env (even empty) replaces it.
func (r *ExecRunner) runInteractive(
	ctx context.Context, dir string, env []string, name string, args ...string,
) error {
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec interactive", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = r.ios.In
	cmd.Stdout = r.ios.Out
	cmd.Stderr = r.ios.ErrOut
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &ExitError{Code: ee.ExitCode(), Err: fmt.Errorf("run %s: %w", name, err)}
		}
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

// RunAssuan feeds stdin to name's standard input and returns its captured
// stdout. stderr is discarded and a non-zero exit is not an error: pinentry-mac
// exits non-zero when the user cancels, which the caller reads as an empty
// result. Only a failure to start the process (e.g. pinentry not installed)
// returns an error. The ask-pass bridge uses it to drive the pinentry Assuan
// exchange.
func (r *ExecRunner) RunAssuan(ctx context.Context, stdin, name string, args ...string) (string, error) {
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec assuan", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return stdout.String(), nil // a cancel still leaves parseable stdout
		}
		return "", fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.String(), nil
}

// OutputStdin feeds stdin to name's standard input and returns its captured
// stdout. Unlike RunAssuan, a non-zero exit is an error with the stderr tail
// folded in — the encrypt pipe must fail loudly, not read a cancel as an empty
// result. The content passes through memory only, never a temp file.
func (r *ExecRunner) OutputStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec output stdin", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("run %s: %w: %s", name, err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.Bytes(), nil
}

// LookPath reports the absolute path of name, with an install hint when the
// program is missing.
func (r *ExecRunner) LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH (install it, e.g. `brew install %s`): %w", name, name, err)
	}
	return path, nil
}
