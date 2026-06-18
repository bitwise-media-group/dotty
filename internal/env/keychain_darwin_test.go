// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build darwin

package env

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"testing"
)

// fakeRunner records the last invocation and returns canned results, standing
// in for the security(1) CLI.
type fakeRunner struct {
	out      []byte
	err      error
	gotName  string
	gotArgs  []string
	callMade bool
}

func (r *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.callMade = true
	r.gotName = name
	r.gotArgs = args
	return r.out, r.err
}

// securityExitErr synthesizes the wrapped *exec.ExitError that ExecRunner.Output
// would return for a security(1) exit code, so isNotFound's errors.As walk is
// exercised against a real exit status.
func securityExitErr(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != code {
		t.Fatalf("could not synthesize exit %d: %v", code, err)
	}
	return fmt.Errorf("run security: %w: stderr tail", err)
}

func TestSecurityKeychainRead(t *testing.T) {
	ctx := context.Background()

	t.Run("success trims trailing newline", func(t *testing.T) {
		r := &fakeRunner{out: []byte(`{"K":"v"}` + "\n")}
		kc := NewKeychain(r)
		got, err := kc.Read(ctx, "aws")
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if string(got) != `{"K":"v"}` {
			t.Errorf("Read = %q, want %q", got, `{"K":"v"}`)
		}
		want := []string{"find-generic-password", "-s", "dotty:aws", "-a", "aws", "-w"}
		if r.gotName != "security" || !reflect.DeepEqual(r.gotArgs, want) {
			t.Errorf("ran %s %v, want security %v", r.gotName, r.gotArgs, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		r := &fakeRunner{err: securityExitErr(t, errSecItemNotFound)}
		if _, err := NewKeychain(r).Read(ctx, "aws"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Read err = %v, want ErrNotFound", err)
		}
	})

	t.Run("other error is surfaced", func(t *testing.T) {
		r := &fakeRunner{err: securityExitErr(t, 1)}
		_, err := NewKeychain(r).Read(ctx, "aws")
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("Read err = %v, want a non-ErrNotFound error", err)
		}
	})
}

func TestSecurityKeychainWrite(t *testing.T) {
	r := &fakeRunner{}
	if err := NewKeychain(r).Write(context.Background(), "aws", []byte(`{"K":"v"}`)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []string{
		"add-generic-password", "-U",
		"-s", "dotty:aws", "-a", "aws",
		"-D", "dotty env", "-w", `{"K":"v"}`,
	}
	if r.gotName != "security" || !reflect.DeepEqual(r.gotArgs, want) {
		t.Errorf("ran %s %v, want security %v", r.gotName, r.gotArgs, want)
	}
}

func TestSecurityKeychainDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		r := &fakeRunner{}
		if err := NewKeychain(r).Delete(ctx, "aws"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		want := []string{"delete-generic-password", "-s", "dotty:aws", "-a", "aws"}
		if r.gotName != "security" || !reflect.DeepEqual(r.gotArgs, want) {
			t.Errorf("ran %s %v, want security %v", r.gotName, r.gotArgs, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		r := &fakeRunner{err: securityExitErr(t, errSecItemNotFound)}
		if err := NewKeychain(r).Delete(ctx, "aws"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete err = %v, want ErrNotFound", err)
		}
	})
}
