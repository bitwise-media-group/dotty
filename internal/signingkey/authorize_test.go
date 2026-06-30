// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// fakeAuthRunner records the ssh invocation and returns a canned error so the
// exit-code mapping can be exercised without a real remote host.
type fakeAuthRunner struct {
	name string
	args []string
	err  error
}

func (f *fakeAuthRunner) RunInteractive(_ context.Context, name string, args ...string) error {
	f.name = name
	f.args = append([]string(nil), args...)
	return f.err
}

func TestBuildAuthorizeScript(t *testing.T) {
	pub := []byte("sk-ssh-ed25519@openssh.com AAAAtest deavon@host\n")

	t.Run("prefixes options and dedups on the bare identity", func(t *testing.T) {
		script := buildAuthorizeScript("~/.ssh/authorized_keys", "no-touch-required", pub)
		wants := []string{
			`f="$HOME/.ssh/authorized_keys"`,
			// The grep needle is the bare algorithm+blob (no options), so an
			// entry already on file without options still counts as present.
			`grep -qF 'sk-ssh-ed25519@openssh.com AAAAtest' "$f"`,
			"exit 3",
			`printf '%s\n' 'no-touch-required sk-ssh-ed25519@openssh.com AAAAtest deavon@host' >> "$f"`,
		}
		for _, want := range wants {
			if !strings.Contains(script, want) {
				t.Errorf("script missing %q\n--- script ---\n%s", want, script)
			}
		}
	})

	t.Run("empty options writes a bare entry", func(t *testing.T) {
		script := buildAuthorizeScript("~/.ssh/authorized_keys", "", pub)
		want := `printf '%s\n' 'sk-ssh-ed25519@openssh.com AAAAtest deavon@host' >> "$f"`
		if !strings.Contains(script, want) {
			t.Errorf("script missing bare line %q\n--- script ---\n%s", want, script)
		}
	})

	t.Run("escapes single quotes in the comment", func(t *testing.T) {
		script := buildAuthorizeScript("~/.ssh/authorized_keys", "",
			[]byte("sk-ssh-ed25519@openssh.com AAAAtest deavon's key"))
		want := `printf '%s\n' 'sk-ssh-ed25519@openssh.com AAAAtest deavon'\''s key' >> "$f"`
		if !strings.Contains(script, want) {
			t.Errorf("script missing escaped line %q\n--- script ---\n%s", want, script)
		}
	})

	t.Run("rewrites the remote path", func(t *testing.T) {
		cases := []struct {
			path string
			want string
		}{
			{"~/.ssh/authorized_keys", `f="$HOME/.ssh/authorized_keys"`},
			{"~", `f="$HOME"`},
			{"/etc/ssh/authorized_keys", `f="/etc/ssh/authorized_keys"`},
			{"~/.ssh/$weird", `f="$HOME/.ssh/\$weird"`},
		}
		for _, c := range cases {
			script := buildAuthorizeScript(c.path, "no-touch-required", pub)
			if !strings.Contains(script, c.want) {
				t.Errorf("path %q: script missing %q\n--- script ---\n%s", c.path, c.want, script)
			}
		}
	})
}

func TestAuthorize(t *testing.T) {
	pub := []byte("sk-ssh-ed25519@openssh.com AAAAtest deavon@host\n")
	const opts = "no-touch-required"

	t.Run("appends via ssh and reports success", func(t *testing.T) {
		r := &fakeAuthRunner{}
		if err := Authorize(context.Background(), r, "user@host", "~/.ssh/authorized_keys", opts, pub); err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if r.name != "ssh" {
			t.Errorf("ran %q, want ssh", r.name)
		}
		if len(r.args) != 2 || r.args[0] != "user@host" {
			t.Fatalf("args = %v, want [user@host <script>]", r.args)
		}
		if want := buildAuthorizeScript("~/.ssh/authorized_keys", opts, pub); r.args[1] != want {
			t.Errorf("script arg = %q, want %q", r.args[1], want)
		}
	})

	t.Run("duplicate exit maps to ErrAlreadyAuthorized", func(t *testing.T) {
		r := &fakeAuthRunner{err: &cli.ExitError{Code: authDuplicateExit}}
		err := Authorize(context.Background(), r, "user@host", "~/.ssh/authorized_keys", opts, pub)
		if !errors.Is(err, ErrAlreadyAuthorized) {
			t.Errorf("err = %v, want ErrAlreadyAuthorized", err)
		}
	})

	t.Run("ssh failure is wrapped, not a duplicate", func(t *testing.T) {
		r := &fakeAuthRunner{err: &cli.ExitError{Code: 255, Err: errors.New("connection refused")}}
		err := Authorize(context.Background(), r, "user@host", "~/.ssh/authorized_keys", opts, pub)
		if err == nil {
			t.Fatal("Authorize() = nil, want error")
		}
		if errors.Is(err, ErrAlreadyAuthorized) {
			t.Error("ssh failure must not map to ErrAlreadyAuthorized")
		}
		if !strings.Contains(err.Error(), "user@host") {
			t.Errorf("err %q should name the host", err)
		}
	})

	t.Run("non-exit error is wrapped, not a duplicate", func(t *testing.T) {
		r := &fakeAuthRunner{err: errors.New("ssh not found")}
		err := Authorize(context.Background(), r, "user@host", "~/.ssh/authorized_keys", opts, pub)
		if err == nil || errors.Is(err, ErrAlreadyAuthorized) {
			t.Errorf("err = %v, want a wrapped non-duplicate error", err)
		}
	})
}
