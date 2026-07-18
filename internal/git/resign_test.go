// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

type interactiveCall struct {
	args     []string
	fileBody string // contents of any -F <file> argument, captured before cleanup
}

type fakeRunner struct {
	output   func(args []string) ([]byte, error)
	runErr   error
	runCalls []interactiveCall
}

func (f *fakeRunner) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	if f.output == nil {
		return nil, nil
	}
	return f.output(args)
}

func (f *fakeRunner) Run(context.Context, string, ...string) error { return nil }

func (f *fakeRunner) RunInteractive(_ context.Context, _ string, args ...string) error {
	c := interactiveCall{args: args}
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-F" {
			if b, err := os.ReadFile(args[i+1]); err == nil {
				c.fileBody = string(b)
			}
		}
	}
	f.runCalls = append(f.runCalls, c)
	return f.runErr
}

// TestRebaseArgs is the regression table for the exact git rebase invocation:
// force-rebase always, --gpg-sign vs the --exec re-entry, and the range target.
func TestRebaseArgs(t *testing.T) {
	const exe = "/usr/local/bin/dotty"
	wantExec := "'/usr/local/bin/dotty' git resign --amend-head"

	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "base, sign only",
			opts: Options{Base: "HEAD~3"},
			want: []string{"rebase", "--force-rebase", "--gpg-sign", "HEAD~3"},
		},
		{
			name: "root, sign only",
			opts: Options{Root: true},
			want: []string{"rebase", "--force-rebase", "--gpg-sign", "--root"},
		},
		{
			name: "base, reset author",
			opts: Options{Base: "abc123", ResetAuthor: true, Exe: exe},
			want: []string{"rebase", "--force-rebase", "--exec", wantExec, "abc123"},
		},
		{
			name: "root, reset author",
			opts: Options{Root: true, ResetAuthor: true, Exe: exe},
			want: []string{"rebase", "--force-rebase", "--exec", wantExec, "--root"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RebaseArgs(tt.opts); !slices.Equal(got, tt.want) {
				t.Errorf("RebaseArgs() =\n%v\nwant\n%v", got, tt.want)
			}
		})
	}
}

func TestRewriteTrailers(t *testing.T) {
	const (
		oldName, oldEmail = "Old Name", "old@example.com"
		newName, newEmail = "New Name", "new@example.com"
	)
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{
			name: "trailer with old identity is rewritten",
			msg:  "subject\n\nCo-authored-by: Old Name <old@example.com>\n",
			want: "subject\n\nCo-authored-by: New Name <new@example.com>\n",
		},
		{
			name: "multiple trailers rewritten, others kept",
			msg: "subject\n\n" +
				"Signed-off-by: Old Name <old@example.com>\n" +
				"Co-authored-by: Other Dev <other@example.com>\n" +
				"Reviewed-by: Old Name <old@example.com>\n",
			want: "subject\n\n" +
				"Signed-off-by: New Name <new@example.com>\n" +
				"Co-authored-by: Other Dev <other@example.com>\n" +
				"Reviewed-by: New Name <new@example.com>\n",
		},
		{
			name: "body prose mentioning the identity is untouched",
			msg:  "Old Name <old@example.com> first wrote this\n\nSigned-off-by: Old Name <old@example.com>\n",
			want: "Old Name <old@example.com> first wrote this\n\nSigned-off-by: New Name <new@example.com>\n",
		},
		{
			name: "no occurrence is unchanged",
			msg:  "subject\n\nSigned-off-by: Someone Else <else@example.com>\n",
			want: "subject\n\nSigned-off-by: Someone Else <else@example.com>\n",
		},
		{
			name: "email-only match (different name) is not rewritten",
			msg:  "subject\n\nCo-authored-by: Different Name <old@example.com>\n",
			want: "subject\n\nCo-authored-by: Different Name <old@example.com>\n",
		},
		{
			name: "empty email with old name is populated",
			msg:  "subject\n\nSigned-off-by: Old Name <>\n",
			want: "subject\n\nSigned-off-by: New Name <new@example.com>\n",
		},
		{
			name: "empty email with new name is populated",
			msg:  "subject\n\nSigned-off-by: New Name <>\n",
			want: "subject\n\nSigned-off-by: New Name <new@example.com>\n",
		},
		{
			name: "empty email with unrelated name is untouched",
			msg:  "subject\n\nSigned-off-by: Someone Else <>\n",
			want: "subject\n\nSigned-off-by: Someone Else <>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteTrailers(tt.msg, oldName, oldEmail, newName, newEmail)
			if got != tt.want {
				t.Errorf("rewriteTrailers() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}

	t.Run("empty old author email still rewrites the trailer", func(t *testing.T) {
		msg := "subject\n\nSigned-off-by: Old Name <>\n"
		want := "subject\n\nSigned-off-by: New Name <new@example.com>\n"
		if got := rewriteTrailers(msg, oldName, "", newName, newEmail); got != want {
			t.Errorf("rewriteTrailers() =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("identical identity is a no-op", func(t *testing.T) {
		msg := "subject\n\nSigned-off-by: Old Name <old@example.com>\n"
		if got := rewriteTrailers(msg, oldName, oldEmail, oldName, oldEmail); got != msg {
			t.Errorf("rewriteTrailers() = %q, want unchanged", got)
		}
	})
}

func TestResign(t *testing.T) {
	f := &fakeRunner{}
	opts := Options{Base: "HEAD~2"}
	if err := Resign(context.Background(), f, opts); err != nil {
		t.Fatalf("Resign() error: %v", err)
	}
	if len(f.runCalls) != 1 {
		t.Fatalf("RunInteractive called %d times, want 1", len(f.runCalls))
	}
	if want := RebaseArgs(opts); !slices.Equal(f.runCalls[0].args, want) {
		t.Errorf("rebase args = %v, want %v", f.runCalls[0].args, want)
	}
}

// amendOutput answers the read-only queries AmendHead makes, with the given
// commit message.
func amendOutput(msg string) func([]string) ([]byte, error) {
	return func(args []string) ([]byte, error) {
		switch {
		case len(args) >= 3 && args[0] == "log" && args[2] == "--format=%an%x00%ae":
			return []byte("Old Name\x00old@example.com\n"), nil
		case args[0] == "config" && args[len(args)-1] == "user.name":
			return []byte("New Name\n"), nil
		case args[0] == "config" && args[len(args)-1] == "user.email":
			return []byte("new@example.com\n"), nil
		case len(args) >= 3 && args[0] == "log" && args[2] == "--format=%B":
			return []byte(msg), nil
		}
		return nil, fmt.Errorf("unexpected git %v", args)
	}
}

func TestAmendHead(t *testing.T) {
	t.Run("rewrites trailer and amends from a message file", func(t *testing.T) {
		f := &fakeRunner{output: amendOutput("subject\n\nCo-authored-by: Old Name <old@example.com>\n")}
		if err := AmendHead(context.Background(), f); err != nil {
			t.Fatalf("AmendHead() error: %v", err)
		}
		if len(f.runCalls) != 1 {
			t.Fatalf("RunInteractive called %d times, want 1", len(f.runCalls))
		}
		got := f.runCalls[0].args
		wantFlags := []string{
			"commit", "--amend", "--reset-author", "--no-verify",
			"--gpg-sign", "--cleanup=verbatim", "-F",
		}
		for _, want := range wantFlags {
			if !slices.Contains(got, want) {
				t.Errorf("amend args %v missing %q", got, want)
			}
		}
		if slices.Contains(got, "--no-edit") {
			t.Errorf("amend args %v should not use --no-edit when the message changed", got)
		}
		if want := "subject\n\nCo-authored-by: New Name <new@example.com>\n"; f.runCalls[0].fileBody != want {
			t.Errorf("message file =\n%q\nwant\n%q", f.runCalls[0].fileBody, want)
		}
	})

	t.Run("no trailer change keeps the message with --no-edit", func(t *testing.T) {
		f := &fakeRunner{output: amendOutput("subject\n\nSigned-off-by: Someone Else <else@example.com>\n")}
		if err := AmendHead(context.Background(), f); err != nil {
			t.Fatalf("AmendHead() error: %v", err)
		}
		got := f.runCalls[0].args
		if !slices.Contains(got, "--no-edit") {
			t.Errorf("amend args %v should use --no-edit when the message is unchanged", got)
		}
		if slices.Contains(got, "-F") {
			t.Errorf("amend args %v should not pass -F when the message is unchanged", got)
		}
	})
}

func TestEnsureIdentity(t *testing.T) {
	t.Run("ok when both set", func(t *testing.T) {
		f := &fakeRunner{output: func(args []string) ([]byte, error) {
			return []byte("set\n"), nil
		}}
		if err := EnsureIdentity(context.Background(), f); err != nil {
			t.Errorf("EnsureIdentity() error: %v", err)
		}
	})
	t.Run("errors when a key is unset", func(t *testing.T) {
		f := &fakeRunner{output: func(args []string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "user.email") {
				return nil, fmt.Errorf("exit status 1")
			}
			return []byte("New Name\n"), nil
		}}
		if err := EnsureIdentity(context.Background(), f); err == nil {
			t.Error("EnsureIdentity() = nil, want error for unset user.email")
		}
	})
}
