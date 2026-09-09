// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package envmigrate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/fnox"
)

// fakeKeychain is the in-memory legacy store, one JSON item per namespace.
type fakeKeychain struct {
	items map[string]string
}

func (f *fakeKeychain) Read(_ context.Context, ns string) ([]byte, error) {
	v, ok := f.items[ns]
	if !ok {
		return nil, env.ErrNotFound
	}
	return []byte(v), nil
}

func (f *fakeKeychain) Delete(_ context.Context, ns string) error {
	if _, ok := f.items[ns]; !ok {
		return env.ErrNotFound
	}
	delete(f.items, ns)
	return nil
}

func (f *fakeKeychain) List(context.Context) ([]string, error) {
	names := make([]string, 0, len(f.items))
	for ns := range f.items {
		names = append(names, ns)
	}
	return names, nil
}

// write is one recorded fnox write: a value set or a default.
type write struct {
	target   fnox.Target
	provider string
	key      string
	value    string
	isDef    bool
}

// fakeFnox records writes and answers Keys from existing; failKeys makes
// Set fail for those keys.
type fakeFnox struct {
	writes   []write
	existing map[string][]string // Target.String() -> keys
	failKeys []string
}

func (f *fakeFnox) Set(_ context.Context, t fnox.Target, provider, key string, value []byte) error {
	for _, k := range f.failKeys {
		if k == key {
			return errors.New("fake set failure for " + key)
		}
	}
	f.writes = append(f.writes, write{target: t, provider: provider, key: key, value: string(value)})
	return nil
}

func (f *fakeFnox) SetDefault(_ context.Context, t fnox.Target, key, def string) error {
	f.writes = append(f.writes, write{target: t, key: key, value: def, isDef: true})
	return nil
}

func (f *fakeFnox) Keys(_ context.Context, t fnox.Target) ([]string, error) {
	return f.existing[t.String()], nil
}

// newMigrator wires a migrator over fakes with a captured stderr.
func newMigrator(t *testing.T, kc *fakeKeychain, f *fakeFnox) (*Migrator, *bytes.Buffer) {
	t.Helper()
	var errOut bytes.Buffer
	ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut}
	return New(ios, env.NewStore(kc), f), &errOut
}

func TestTarget(t *testing.T) {
	if got := Target("default"); got != (fnox.Target{Global: true}) {
		t.Errorf("Target(default) = %+v, want global top-level", got)
	}
	if got := Target("aws"); got != (fnox.Target{Global: true, Profile: "aws"}) {
		t.Errorf("Target(aws) = %+v, want global profile aws", got)
	}
}

func TestNamespaces(t *testing.T) {
	ctx := context.Background()
	kc := &fakeKeychain{items: map[string]string{
		"default": `{"TOKEN":"tok"}`,
		"aws":     `{"KEY":"k","SECRET":"s"}`,
	}}
	f := &fakeFnox{}
	m, _ := newMigrator(t, kc, f)

	report, err := m.Namespaces(ctx, Options{Provider: "dotty-age"})
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if report.Migrated != 3 || report.Skipped != 0 || report.Failed != 0 || len(report.Purged) != 0 {
		t.Errorf("report = %+v, want 3 migrated", report)
	}
	// Namespaces sorted, keys sorted within; default has no profile.
	want := []write{
		{target: fnox.Target{Global: true, Profile: "aws"}, provider: "dotty-age", key: "KEY", value: "k"},
		{target: fnox.Target{Global: true, Profile: "aws"}, provider: "dotty-age", key: "SECRET", value: "s"},
		{target: fnox.Target{Global: true}, provider: "dotty-age", key: "TOKEN", value: "tok"},
	}
	if !reflect.DeepEqual(f.writes, want) {
		t.Errorf("writes = %+v, want %+v", f.writes, want)
	}
	if _, ok := kc.items["aws"]; !ok {
		t.Error("namespace purged without --purge")
	}
}

func TestNamespacesSkipsExistingUnlessForce(t *testing.T) {
	ctx := context.Background()
	kc := &fakeKeychain{items: map[string]string{"aws": `{"KEY":"k","NEW":"n"}`}}
	f := &fakeFnox{existing: map[string][]string{Target("aws").String(): {"KEY"}}}
	m, _ := newMigrator(t, kc, f)

	report, err := m.Namespaces(ctx, Options{Namespaces: []string{"aws"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 || report.Skipped != 1 || len(f.writes) != 1 || f.writes[0].key != "NEW" {
		t.Errorf("report = %+v, writes = %+v; want NEW migrated and KEY skipped", report, f.writes)
	}

	f.writes = nil
	report, err = m.Namespaces(ctx, Options{Namespaces: []string{"aws"}, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 2 || report.Skipped != 0 || len(f.writes) != 2 {
		t.Errorf("forced report = %+v, writes = %+v; want both migrated", report, f.writes)
	}
}

func TestNamespacesPurge(t *testing.T) {
	ctx := context.Background()
	t.Run("failure withholds purge", func(t *testing.T) {
		kc := &fakeKeychain{items: map[string]string{"aws": `{"BAD":"x","GOOD":"y"}`}}
		f := &fakeFnox{failKeys: []string{"BAD"}}
		m, errOut := newMigrator(t, kc, f)
		report, err := m.Namespaces(ctx, Options{Purge: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 1 || report.Failed != 1 || len(report.Purged) != 0 {
			t.Errorf("report = %+v, want 1 migrated, 1 failed, nothing purged", report)
		}
		if _, ok := kc.items["aws"]; !ok {
			t.Error("namespace purged despite a failure")
		}
		if !strings.Contains(errOut.String(), "fake set failure for BAD") {
			t.Errorf("stderr = %q, want the set failure warned", errOut.String())
		}
	})
	t.Run("purge after success", func(t *testing.T) {
		kc := &fakeKeychain{items: map[string]string{"aws": `{"GOOD":"y"}`, "ci": `{"T":"t"}`}}
		m, _ := newMigrator(t, kc, &fakeFnox{})
		report, err := m.Namespaces(ctx, Options{Purge: true})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(report.Purged, []string{"aws", "ci"}) || len(kc.items) != 0 {
			t.Errorf("report.Purged = %v, items left = %v; want both purged", report.Purged, kc.items)
		}
	})
	t.Run("dry run never purges", func(t *testing.T) {
		kc := &fakeKeychain{items: map[string]string{"aws": `{"GOOD":"y"}`}}
		f := &fakeFnox{}
		m, _ := newMigrator(t, kc, f)
		report, err := m.Namespaces(ctx, Options{Purge: true, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 1 || len(report.Purged) != 0 || len(f.writes) != 0 || len(kc.items) != 1 {
			t.Errorf("dry run: report = %+v, writes = %v, items = %v", report, f.writes, kc.items)
		}
	})
}

func TestNamespacesWarnsOnWhitespace(t *testing.T) {
	kc := &fakeKeychain{items: map[string]string{"aws": `{"PAD":" x "}`}}
	m, errOut := newMigrator(t, kc, &fakeFnox{})
	if _, err := m.Namespaces(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "trims surrounding whitespace") {
		t.Errorf("stderr = %q, want whitespace warning", errOut.String())
	}
}

func TestFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	src := "PORT=8080\n" +
		"TOKEN={{ dotty://ci/TOKEN }}\n" +
		"BARE={{ BARE }}\n" +
		"BAD=\"open\n" +
		"MISSING={{ dotty://ci/NOPE }}\n" +
		"MIXED=a{{ BARE }}b\n" +
		"LAST=done\n"
	if err := os.WriteFile(tmpl, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	kc := &fakeKeychain{items: map[string]string{
		"ci":      `{"TOKEN":"tok"}`,
		"default": `{"BARE":"bare"}`,
	}}
	f := &fakeFnox{}
	m, errOut := newMigrator(t, kc, f)

	report, err := m.Files(ctx, Options{Files: []string{tmpl}, Provider: "dotty-age"})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if report.Migrated != 4 || report.Failed != 3 {
		t.Errorf("report = %+v, want 4 migrated, 3 failed", report)
	}
	dst := filepath.Join(dir, ProjectConfig)
	pt := fnox.Target{Config: dst}
	want := []write{
		{target: pt, key: "PORT", value: "8080", isDef: true},
		{target: pt, provider: "dotty-age", key: "TOKEN", value: "tok"},
		{target: pt, provider: "dotty-age", key: "BARE", value: "bare"},
		{target: pt, key: "LAST", value: "done", isDef: true},
	}
	if !reflect.DeepEqual(f.writes, want) {
		t.Errorf("writes = %+v, want %+v", f.writes, want)
	}
	// The project file is seeded so fnox -c can write to it; the template
	// stays for the user to delete.
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("fnox.toml not created: %v", err)
	}
	if _, err := os.Stat(tmpl); err != nil {
		t.Errorf("template removed: %v", err)
	}
	for _, sub := range []string{":4: unterminated quote", ":5: ", "key not found", ":6: ", "interpolation"} {
		if !strings.Contains(errOut.String(), sub) {
			t.Errorf("stderr lacks %q:\n%s", sub, errOut.String())
		}
	}
}

func TestFilesSkipsExistingAndDryRun(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	if err := os.WriteFile(tmpl, []byte("A=1\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, ProjectConfig)

	t.Run("dry run writes nothing", func(t *testing.T) {
		f := &fakeFnox{}
		m, _ := newMigrator(t, &fakeKeychain{}, f)
		report, err := m.Files(ctx, Options{Files: []string{tmpl}, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 2 || len(f.writes) != 0 {
			t.Errorf("dry run report = %+v, writes = %v", report, f.writes)
		}
		if _, err := os.Stat(dst); err == nil {
			t.Error("dry run created fnox.toml")
		}
	})
	t.Run("existing keys skipped", func(t *testing.T) {
		if err := os.WriteFile(dst, []byte("[secrets]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		f := &fakeFnox{existing: map[string][]string{dst: {"A"}}}
		m, _ := newMigrator(t, &fakeKeychain{}, f)
		report, err := m.Files(ctx, Options{Files: []string{tmpl}})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 1 || report.Skipped != 1 || len(f.writes) != 1 || f.writes[0].key != "B" {
			t.Errorf("report = %+v, writes = %+v; want only B written", report, f.writes)
		}
	})
}

func TestReportAdd(t *testing.T) {
	var r Report
	r.Add(Report{Migrated: 1, Skipped: 2, Failed: 3, Purged: []string{"a"}})
	r.Add(Report{Migrated: 1, Purged: []string{"b"}})
	want := Report{Migrated: 2, Skipped: 2, Failed: 3, Purged: []string{"a", "b"}}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("Add = %+v, want %+v", r, want)
	}
}
