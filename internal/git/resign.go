// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Runner runs git. Output captures stdout for the small read-only queries;
// RunInteractive wires the full terminal so the rebase shows progress and the
// signing program can prompt, and so a non-zero exit surfaces as *cli.ExitError.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// Options parameterize one resign. Exactly one of Root or Base selects the
// range: Root rebases from the very first commit, Base rebases Base..HEAD.
// ResetAuthor additionally rewrites each commit's author; Exe is the dotty
// binary path re-invoked per commit (only used when ResetAuthor is set).
type Options struct {
	Root        bool
	Base        string
	ResetAuthor bool
	Exe         string
}

// RebaseArgs builds the git rebase argv. --force-rebase recreates every commit
// even when unchanged. Without author reset, --gpg-sign signs each recreated
// commit through git's configured signing program. With author reset, an --exec
// step re-invokes dotty to amend (reset author, rewrite trailers, sign) each
// commit, so --gpg-sign is omitted to avoid a second signature.
func RebaseArgs(o Options) []string {
	args := []string{"rebase", "--force-rebase"}
	if o.ResetAuthor {
		args = append(args, "--exec", execCommand(o.Exe))
	} else {
		args = append(args, "--gpg-sign")
	}
	if o.Root {
		return append(args, "--root")
	}
	return append(args, o.Base)
}

// EnsureIdentity verifies user.name and user.email are set so a --reset-author
// resign fails fast rather than partway through the rebase.
func EnsureIdentity(ctx context.Context, r Runner) error {
	if _, err := configValue(ctx, r, "user.name"); err != nil {
		return err
	}
	if _, err := configValue(ctx, r, "user.email"); err != nil {
		return err
	}
	return nil
}

// CommitCount reports how many commits the resign will rewrite, for the
// confirmation prompt.
func CommitCount(ctx context.Context, r Runner, o Options) (int, error) {
	spec := "HEAD"
	if !o.Root {
		spec = o.Base + "..HEAD"
	}
	out, err := r.Output(ctx, "git", "rev-list", "--count", spec)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse commit count %q: %w", strings.TrimSpace(string(out)), err)
	}
	return n, nil
}

// Resign runs the rebase that re-signs (and optionally re-authors) the range. A
// rebase conflict stops it for the user to resolve with `git rebase --continue`;
// the non-zero exit is proxied via *cli.ExitError.
func Resign(ctx context.Context, r Runner, o Options) error {
	return r.RunInteractive(ctx, "git", RebaseArgs(o)...)
}

// AmendHead is the per-commit rebase --exec step: it resets HEAD's author to
// user.name/user.email, rewrites any trailer naming the original author, and
// re-signs. The original author is read before the amend, while the rebase has
// preserved it.
func AmendHead(ctx context.Context, r Runner) error {
	oldName, oldEmail, err := headAuthor(ctx, r)
	if err != nil {
		return err
	}
	newName, err := configValue(ctx, r, "user.name")
	if err != nil {
		return err
	}
	newEmail, err := configValue(ctx, r, "user.email")
	if err != nil {
		return err
	}
	out, err := r.Output(ctx, "git", "log", "-1", "--format=%B", "HEAD")
	if err != nil {
		return fmt.Errorf("read commit message: %w", err)
	}
	orig := string(out)
	rewritten := rewriteTrailers(orig, oldName, oldEmail, newName, newEmail)

	args := []string{"commit", "--amend", "--reset-author", "--no-verify", "--gpg-sign"}
	if rewritten == orig {
		args = append(args, "--no-edit")
	} else {
		path, cleanup, err := writeTempMessage(rewritten)
		if err != nil {
			return err
		}
		defer cleanup()
		args = append(args, "--cleanup=verbatim", "-F", path)
	}
	return r.RunInteractive(ctx, "git", args...)
}

// trailerLine matches a git trailer ("Key: value", e.g. Co-authored-by:),
// distinguishing trailers from body prose so only trailers are rewritten.
var trailerLine = regexp.MustCompile(`^[A-Za-z][A-Za-z-]*: `)

// rewriteTrailers replaces the "Name <email>" identity of oldName/oldEmail with
// newName/newEmail on every trailer line that contains it, leaving body prose
// and other lines untouched.
func rewriteTrailers(msg, oldName, oldEmail, newName, newEmail string) string {
	if oldName == "" || oldEmail == "" {
		return msg
	}
	oldID := oldName + " <" + oldEmail + ">"
	newID := newName + " <" + newEmail + ">"
	if oldID == newID || !strings.Contains(msg, oldID) {
		return msg
	}
	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		if trailerLine.MatchString(line) && strings.Contains(line, oldID) {
			lines[i] = strings.ReplaceAll(line, oldID, newID)
		}
	}
	return strings.Join(lines, "\n")
}

// headAuthor returns HEAD's author name and email.
func headAuthor(ctx context.Context, r Runner) (name, email string, err error) {
	out, err := r.Output(ctx, "git", "log", "-1", "--format=%an%x00%ae", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("read commit author: %w", err)
	}
	name, email, ok := strings.Cut(strings.TrimRight(string(out), "\n"), "\x00")
	if !ok {
		return "", "", fmt.Errorf("parse commit author %q", string(out))
	}
	return name, email, nil
}

// configValue reads a non-empty git config value, turning an unset key into a
// clear error.
func configValue(ctx context.Context, r Runner, key string) (string, error) {
	out, err := r.Output(ctx, "git", "config", "--get", key)
	if err != nil {
		return "", fmt.Errorf("git config %s is not set: %w", key, err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf("git config %s is empty", key)
	}
	return v, nil
}

// execCommand builds the rebase --exec string that re-invokes dotty to amend
// the current commit. The binary path is shell-quoted because git runs the
// step through the shell.
func execCommand(exe string) string {
	return shellQuote(exe) + " git resign --amend-head"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// writeTempMessage writes content to a temp file and returns its path plus a
// cleanup func that removes it.
func writeTempMessage(content string) (path string, cleanup func(), err error) {
	tmp, err := os.CreateTemp("", "dotty-resign-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	path = tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("close %s: %w", path, err)
	}
	return path, func() { _ = os.Remove(path) }, nil
}
