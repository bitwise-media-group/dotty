// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
)

// TestKeychainRoundTrip exercises the real OS keychain end-to-end against a
// throwaway namespace seeded the way the old verbs wrote it. It is opt-in
// (DOTTY_ENV_KEYCHAIN_IT=1) because it mutates the login keychain — keep it
// out of CI and run it by hand on macOS:
//
//	DOTTY_ENV_KEYCHAIN_IT=1 go test ./internal/env -run TestKeychainRoundTrip -v
func TestKeychainRoundTrip(t *testing.T) {
	if os.Getenv("DOTTY_ENV_KEYCHAIN_IT") == "" {
		t.Skip("set DOTTY_ENV_KEYCHAIN_IT=1 to run the real keychain integration test")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("keychain integration test requires macOS")
	}

	ctx := context.Background()
	store := env.NewStore(env.NewKeychain(cli.NewExecRunner(cli.System(), nil)))
	const ns = "dotty-it-roundtrip"

	seed := exec.CommandContext(ctx, "security", "add-generic-password", "-U",
		"-s", "dotty:"+ns, "-a", ns, "-D", "dotty env", "-w", `{"TOKEN":"hunter2","MULTI":"a\nb"}`)
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seed keychain item: %v: %s", err, out)
	}
	t.Cleanup(func() {
		if err := store.DeleteNamespace(ctx, ns); err != nil {
			t.Errorf("cleanup DeleteNamespace: %v", err)
		}
	})

	if got, err := store.Get(ctx, ns, "TOKEN"); err != nil || got != "hunter2" {
		t.Fatalf("Get TOKEN = %q (err %v), want hunter2", got, err)
	}
	if got, err := store.Get(ctx, ns, "MULTI"); err != nil || got != "a\nb" {
		t.Fatalf("Get MULTI = %q (err %v), want a\\nb", got, err)
	}
	keys, err := store.Keys(ctx, ns)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 2 || keys[0] != "MULTI" || keys[1] != "TOKEN" {
		t.Fatalf("Keys = %v, want [MULTI TOKEN]", keys)
	}
	namespaces, err := store.Namespaces(ctx)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if !slices.Contains(namespaces, ns) {
		t.Fatalf("Namespaces = %v, want it to include %s", namespaces, ns)
	}

	if err := store.DeleteNamespace(ctx, ns); err != nil {
		t.Fatalf("DeleteNamespace: %v", err)
	}
	if _, err := store.Get(ctx, ns, "TOKEN"); !errors.Is(err, env.ErrKeyNotFound) {
		t.Fatalf("Get after delete err = %v, want ErrKeyNotFound", err)
	}
}
