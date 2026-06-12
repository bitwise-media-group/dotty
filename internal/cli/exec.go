// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	r.log.LogAttrs(ctx, slog.LevelDebug, "exec interactive", slog.String("cmd", name), slog.Any("args", args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = r.ios.In
	cmd.Stdout = r.ios.Out
	cmd.Stderr = r.ios.ErrOut
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &ExitError{Code: ee.ExitCode(), Err: fmt.Errorf("run %s: %w", name, err)}
		}
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
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
