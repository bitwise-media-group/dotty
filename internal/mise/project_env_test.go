// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectEnvKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectConfig)
	doc := `[tools]
terraform = "1.10"

[env]
# a comment
_.file = ".env"
PORT = "8080"
"QUOTED" = 'x'
REGION = "eu-west-2" # trailing

[tasks.plan]
run = "terraform plan"
`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	p := NewProjectEnv(&fakeRunner{}, "")

	got, err := p.Keys(path)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if want := []string{"PORT", "QUOTED", "REGION"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Keys = %v, want %v", got, want)
	}

	got, err = p.Keys(filepath.Join(dir, "absent.toml"))
	if err != nil || got != nil {
		t.Errorf("Keys(missing) = %v, %v; want none", got, err)
	}
}

func TestProjectEnvSetAndTrust(t *testing.T) {
	ctx := context.Background()
	r := &fakeRunner{}
	p := NewProjectEnv(r, "/bin/mise")

	if err := p.Set(ctx, "/repo/mise.toml", "URL", "a=b c"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := p.Trust(ctx, "/repo/mise.toml"); err != nil {
		t.Fatalf("Trust: %v", err)
	}
	want := [][]string{
		{"/bin/mise", "set", "--file", "/repo/mise.toml", "URL=a=b c"},
		{"/bin/mise", "trust", "/repo/mise.toml"},
	}
	for i, c := range r.calls {
		if c.env != nil {
			t.Errorf("call %d env = %v, want none (no profile retargeting)", i, c.env)
		}
		if i >= len(want) || !reflect.DeepEqual(c.argv, want[i]) {
			t.Errorf("call %d = %v, want %v", i, c.argv, want)
		}
	}

	r.errs = map[string]error{"set --file /repo/mise.toml K=v": errors.New("boom")}
	err := p.Set(ctx, "/repo/mise.toml", "K", "v")
	if err == nil || !strings.Contains(err.Error(), "set K in /repo/mise.toml") {
		t.Errorf("Set error = %v, want it wrapped with the key and file", err)
	}
}

func TestExpands(t *testing.T) {
	for value, want := range map[string]bool{
		"plain":              false,
		"a@b.iam.example":    false,
		"$HOME/x":            true,
		"{{ env.HOME }}":     true,
		"{% raw %}x":         true,
		"{# comment #}":      true,
		"single { brace }":   false,
		"postgres://h:5/db":  false,
		"price-is-5-dollars": false,
	} {
		if got := Expands(value); got != want {
			t.Errorf("Expands(%q) = %v, want %v", value, got, want)
		}
	}
}
