// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fnox

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// call is one recorded runner invocation.
type call struct {
	name  string
	args  []string
	stdin string
	env   []string
}

// fakeRunner records every invocation and answers each from outputs in
// order (the last output repeats), standing in for the fnox binary.
type fakeRunner struct {
	calls   []call
	outputs []string
	err     error
	missing []string
}

func (r *fakeRunner) next() []byte {
	if len(r.outputs) == 0 {
		return nil
	}
	out := r.outputs[0]
	if len(r.outputs) > 1 {
		r.outputs = r.outputs[1:]
	}
	return []byte(out)
}

func (r *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{name: name, args: args})
	return r.next(), r.err
}

func (r *fakeRunner) OutputEnv(_ context.Context, extraEnv []string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{name: name, args: args, env: extraEnv})
	return r.next(), r.err
}

func (r *fakeRunner) OutputStdin(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{name: name, args: args, stdin: string(stdin)})
	return r.next(), r.err
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	for _, m := range r.missing {
		if m == name {
			return "", errors.New(name + " not found in PATH")
		}
	}
	return "/fake/" + name, nil
}

const globalCfg = "/home/me/.config/fnox/config.toml"

// targets are the three shapes every verb must address correctly.
var targets = []struct {
	name   string
	t      Target
	prefix []string // the global flags before the verb
	global bool
}{
	{name: "global top-level", t: Target{Global: true}, prefix: []string{"-c", globalCfg}, global: true},
	{name: "global profile", t: Target{Global: true, Profile: "aws"},
		prefix: []string{"-c", globalCfg, "-P", "aws"}, global: true},
	{name: "project file", t: Target{Config: "/repo/fnox.toml"}, prefix: []string{"-c", "/repo/fnox.toml"}},
}

// withG appends -g for global targets.
func withG(global bool, verb string) []string {
	if global {
		return []string{verb, "-g"}
	}
	return []string{verb}
}

func TestSetArgs(t *testing.T) {
	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			c := New(r, globalCfg)
			if err := c.Set(context.Background(), tt.t, "dotty-age", "KEY", []byte("s3cret\n")); err != nil {
				t.Fatalf("Set: %v", err)
			}
			want := append(append(append([]string{}, tt.prefix...), withG(tt.global, "set")...), "-p", "dotty-age", "KEY")
			if len(r.calls) != 1 || r.calls[0].name != Bin || !reflect.DeepEqual(r.calls[0].args, want) {
				t.Errorf("calls = %+v, want fnox %v", r.calls, want)
			}
			// The value travels on stdin, never in argv.
			if r.calls[0].stdin != "s3cret\n" {
				t.Errorf("stdin = %q, want the value", r.calls[0].stdin)
			}
		})
	}
	t.Run("no provider", func(t *testing.T) {
		r := &fakeRunner{}
		if err := New(r, globalCfg).Set(context.Background(), Target{Global: true}, "", "KEY", []byte("v")); err != nil {
			t.Fatal(err)
		}
		want := []string{"-c", globalCfg, "set", "-g", "KEY"}
		if !reflect.DeepEqual(r.calls[0].args, want) {
			t.Errorf("args = %v, want %v", r.calls[0].args, want)
		}
	})
}

func TestSetDefaultArgs(t *testing.T) {
	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			if err := New(r, globalCfg).SetDefault(context.Background(), tt.t, "PORT", "8080"); err != nil {
				t.Fatalf("SetDefault: %v", err)
			}
			want := append(append(append([]string{}, tt.prefix...), withG(tt.global, "set")...), "--default=8080", "PORT")
			if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, want) {
				t.Errorf("calls = %+v, want fnox %v", r.calls, want)
			}
			if r.calls[0].stdin != "" {
				t.Errorf("stdin = %q, want nothing for a metadata-only set", r.calls[0].stdin)
			}
		})
	}
}

func TestRemoveArgs(t *testing.T) {
	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			if err := New(r, globalCfg).Remove(context.Background(), tt.t, "KEY"); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			want := append(append(append([]string{}, tt.prefix...), withG(tt.global, "remove")...), "KEY")
			if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, want) {
				t.Errorf("calls = %+v, want fnox %v", r.calls, want)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	ctx := context.Background()
	t.Run("global top-level", func(t *testing.T) {
		r := &fakeRunner{outputs: []string{"B\nA\n"}}
		got, err := New(r, globalCfg).Keys(ctx, Target{Global: true})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"B", "A"}) {
			t.Errorf("Keys = %v", got)
		}
		want := []string{"-c", globalCfg, "--no-defaults", "list", "--complete"}
		if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, want) || r.calls[0].env != nil {
			t.Errorf("calls = %+v, want fnox %v with no env", r.calls, want)
		}
	})
	t.Run("global profile consults profiles first", func(t *testing.T) {
		r := &fakeRunner{outputs: []string{"default\naws\n", "KEY\n"}}
		got, err := New(r, globalCfg).Keys(ctx, Target{Global: true, Profile: "aws"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"KEY"}) {
			t.Errorf("Keys = %v", got)
		}
		wantProfiles := []string{"-c", globalCfg, "profiles", "--complete"}
		wantList := []string{"-c", globalCfg, "-P", "aws", "--no-defaults", "list", "--complete"}
		if len(r.calls) != 2 || !reflect.DeepEqual(r.calls[0].args, wantProfiles) ||
			!reflect.DeepEqual(r.calls[1].args, wantList) {
			t.Errorf("calls = %+v, want %v then %v", r.calls, wantProfiles, wantList)
		}
	})
	t.Run("missing profile has no keys", func(t *testing.T) {
		r := &fakeRunner{outputs: []string{"default\n"}}
		got, err := New(r, globalCfg).Keys(ctx, Target{Global: true, Profile: "aws"})
		if err != nil || got != nil || len(r.calls) != 1 {
			t.Errorf("Keys = %v, %v (calls %d); want nil, nil after one call", got, err, len(r.calls))
		}
	})
	t.Run("project file is isolated from the global config", func(t *testing.T) {
		r := &fakeRunner{outputs: []string{"KEY\n"}}
		got, err := New(r, globalCfg).Keys(ctx, Target{Config: "/repo/fnox.toml"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"KEY"}) {
			t.Errorf("Keys = %v", got)
		}
		want := []string{"-c", "/repo/fnox.toml", "--no-defaults", "list", "--complete"}
		if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, want) {
			t.Errorf("calls = %+v, want fnox %v", r.calls, want)
		}
		if len(r.calls[0].env) != 1 || !strings.HasPrefix(r.calls[0].env[0], "FNOX_CONFIG_DIR=") {
			t.Errorf("env = %v, want FNOX_CONFIG_DIR pointed at a scratch dir", r.calls[0].env)
		}
	})
}

func TestProviders(t *testing.T) {
	t.Run("absent global config has none", func(t *testing.T) {
		r := &fakeRunner{}
		got, err := New(r, t.TempDir()+"/nope/config.toml").Providers(context.Background())
		if err != nil || got != nil || len(r.calls) != 0 {
			t.Errorf("Providers = %v, %v (calls %d); want nil without running fnox", got, err, len(r.calls))
		}
	})
	t.Run("lists names", func(t *testing.T) {
		dir := t.TempDir()
		cfg := dir + "/config.toml"
		if err := writeFile(cfg, "env = \"exec\"\n"); err != nil {
			t.Fatal(err)
		}
		r := &fakeRunner{outputs: []string{"dotty-age\ndotty-keychain\n"}}
		got, err := New(r, cfg).Providers(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"dotty-age", "dotty-keychain"}) {
			t.Errorf("Providers = %v", got)
		}
		want := []string{"-c", cfg, "provider", "list"}
		if !reflect.DeepEqual(r.calls[0].args, want) {
			t.Errorf("args = %v, want %v", r.calls[0].args, want)
		}
	})
}

// TestSetKeychainItem pins the set-then-remove dance: the item is written
// through a scratch secret in the global config that is dropped right after.
func TestSetKeychainItem(t *testing.T) {
	r := &fakeRunner{}
	err := New(r, globalCfg).SetKeychainItem(context.Background(),
		"dotty-keychain", "dotty-age-key", []byte("AGE-SECRET-KEY-1x"))
	if err != nil {
		t.Fatalf("SetKeychainItem: %v", err)
	}
	wantSet := []string{"-c", globalCfg, "set", "-g", "-p", "dotty-keychain", "-k", "dotty-age-key", scratchKey}
	wantRemove := []string{"-c", globalCfg, "remove", "-g", scratchKey}
	if len(r.calls) != 2 || !reflect.DeepEqual(r.calls[0].args, wantSet) ||
		!reflect.DeepEqual(r.calls[1].args, wantRemove) {
		t.Fatalf("calls = %+v, want %v then %v", r.calls, wantSet, wantRemove)
	}
	if r.calls[0].stdin != "AGE-SECRET-KEY-1x" {
		t.Errorf("stdin = %q, want the identity", r.calls[0].stdin)
	}
}

func TestEnsure(t *testing.T) {
	if err := New(&fakeRunner{}, globalCfg).Ensure(); err != nil {
		t.Errorf("Ensure with fnox present = %v", err)
	}
	err := New(&fakeRunner{missing: []string{Bin}}, globalCfg).Ensure()
	if err == nil || !strings.Contains(err.Error(), "dotty packages sync") {
		t.Errorf("Ensure without fnox = %v, want install hint", err)
	}
}

func TestTargetString(t *testing.T) {
	tests := []struct {
		t    Target
		want string
	}{
		{Target{Global: true}, "the global config"},
		{Target{Global: true, Profile: "aws"}, "the global config (profile aws)"},
		{Target{Config: "/repo/fnox.toml"}, "/repo/fnox.toml"},
		{Target{Config: "/repo/fnox.toml", Profile: "ci"}, "/repo/fnox.toml (profile ci)"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}
