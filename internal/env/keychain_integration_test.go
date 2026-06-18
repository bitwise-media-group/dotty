// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
)

// TestKeychainRoundTrip exercises the real OS keychain end-to-end against a
// throwaway namespace. It is opt-in (DOTTY_ENV_KEYCHAIN_IT=1) because it mutates
// the login keychain and cannot run inside a sandbox — keep it out of CI and
// run it by hand on macOS:
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

	t.Cleanup(func() {
		if err := store.DeleteNamespace(ctx, ns); err != nil {
			t.Errorf("cleanup DeleteNamespace: %v", err)
		}
	})

	if err := store.Set(ctx, ns, "TOKEN", "hunter2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Set(ctx, ns, "MULTI", "a\nb"); err != nil {
		t.Fatalf("Set multiline: %v", err)
	}

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

	found, err := store.Delete(ctx, ns, "TOKEN")
	if err != nil || !found {
		t.Fatalf("Delete TOKEN = (%v, %v), want (true, nil)", found, err)
	}
	if keys, _ := store.Keys(ctx, ns); len(keys) != 1 || keys[0] != "MULTI" {
		t.Fatalf("Keys after delete = %v, want [MULTI]", keys)
	}
}
