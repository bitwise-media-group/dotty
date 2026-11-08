// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package envmigrate

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// write is one recorded fnox write.
type write struct {
	target   fnox.Target
	provider string
	key      string
	value    string
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

func (f *fakeFnox) Keys(_ context.Context, t fnox.Target) ([]string, error) {
	return f.existing[t.String()], nil
}

// envVar is one recorded mise variable write.
type envVar struct {
	path  string
	key   string
	value string
}

// fakeEnv records mise writes and trusts, answering Keys from existing.
// Like mise set, a write creates the file it targets.
type fakeEnv struct {
	sets     []envVar
	trusted  []string
	existing map[string][]string // path -> keys
	failKeys []string
}

func (f *fakeEnv) Keys(path string) ([]string, error) { return f.existing[path], nil }

func (f *fakeEnv) Set(_ context.Context, path, key, value string) error {
	if slices.Contains(f.failKeys, key) {
		return errors.New("fake mise failure for " + key)
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(path, []byte("[env]\n"), 0o600); err != nil {
			return err
		}
	}
	f.sets = append(f.sets, envVar{path: path, key: key, value: value})
	return nil
}

func (f *fakeEnv) Trust(_ context.Context, path string) error {
	f.trusted = append(f.trusted, path)
	return nil
}

// newMigrator wires a migrator over fakes with a captured stderr.
func newMigrator(t *testing.T, kc *fakeKeychain, f *fakeFnox) (*Migrator, *bytes.Buffer) {
	t.Helper()
	m, _, errOut := newMigratorEnv(t, kc, f)
	return m, errOut
}

// newMigratorEnv is newMigrator that also returns the mise fake.
func newMigratorEnv(t *testing.T, kc *fakeKeychain, f *fakeFnox) (*Migrator, *fakeEnv, *bytes.Buffer) {
	t.Helper()
	var errOut bytes.Buffer
	ios := cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut}
	e := &fakeEnv{}
	return New(ios, env.NewStore(kc), f, e), e, &errOut
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
	m, e, errOut := newMigratorEnv(t, kc, f)

	report, err := m.Files(ctx, Options{Files: []string{tmpl}, Provider: "dotty-age"})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if report.Migrated != 4 || report.Failed != 3 {
		t.Errorf("report = %+v, want 4 migrated, 3 failed", report)
	}

	// Literals land in mise, references in fnox.
	miseDst := filepath.Join(dir, "mise.toml")
	wantEnv := []envVar{{miseDst, "PORT", "8080"}, {miseDst, "LAST", "done"}}
	if !reflect.DeepEqual(e.sets, wantEnv) {
		t.Errorf("mise sets = %+v, want %+v", e.sets, wantEnv)
	}
	if !reflect.DeepEqual(e.trusted, []string{miseDst}) {
		t.Errorf("trusted = %v, want the mise.toml dotty created", e.trusted)
	}
	dst := filepath.Join(dir, ProjectConfig)
	pt := fnox.Target{Config: dst}
	want := []write{
		{target: pt, provider: "dotty-age", key: "TOKEN", value: "tok"},
		{target: pt, provider: "dotty-age", key: "BARE", value: "bare"},
	}
	if !reflect.DeepEqual(f.writes, want) {
		t.Errorf("fnox writes = %+v, want %+v", f.writes, want)
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

func TestFilesPlainOnly(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	if err := os.WriteFile(tmpl, []byte("SA=apply@p.iam.gserviceaccount.com\nHOMEISH=$HOME/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeFnox{}
	m, e, errOut := newMigratorEnv(t, &fakeKeychain{}, f)

	report, err := m.Files(ctx, Options{Files: []string{tmpl}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 2 || len(e.sets) != 2 || len(f.writes) != 0 {
		t.Errorf("report = %+v, mise sets = %+v, fnox writes = %+v; want both in mise only", report, e.sets, f.writes)
	}
	if _, err := os.Stat(filepath.Join(dir, ProjectConfig)); err == nil {
		t.Error("fnox.toml created for a template without secrets")
	}
	if !strings.Contains(errOut.String(), ":2: mise expands") || strings.Contains(errOut.String(), ":1: mise expands") {
		t.Errorf("stderr = %q, want only line 2 warned about expansion", errOut.String())
	}
}

func TestFilesKeepsTrustOfExistingMiseConfig(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	if err := os.WriteFile(tmpl, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, e, _ := newMigratorEnv(t, &fakeKeychain{}, &fakeFnox{})
	if _, err := m.Files(context.Background(), Options{Files: []string{tmpl}}); err != nil {
		t.Fatal(err)
	}
	if len(e.sets) != 1 || len(e.trusted) != 0 {
		t.Errorf("sets = %+v, trusted = %v; want the write but no trust for a file dotty did not create", e.sets, e.trusted)
	}
}

func TestFilesSkipsExistingAndDryRun(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	if err := os.WriteFile(tmpl, []byte("A=1\nB=2\nS={{ dotty://ci/S }}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kc := &fakeKeychain{items: map[string]string{"ci": `{"S":"s"}`}}
	dst := filepath.Join(dir, ProjectConfig)
	miseDst := filepath.Join(dir, "mise.toml")

	t.Run("dry run writes nothing", func(t *testing.T) {
		f := &fakeFnox{}
		m, e, _ := newMigratorEnv(t, kc, f)
		report, err := m.Files(ctx, Options{Files: []string{tmpl}, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 3 || len(f.writes) != 0 || len(e.sets) != 0 || len(e.trusted) != 0 {
			t.Errorf("dry run report = %+v, fnox = %v, mise = %v", report, f.writes, e.sets)
		}
		for _, p := range []string{dst, miseDst} {
			if _, err := os.Stat(p); err == nil {
				t.Errorf("dry run created %s", p)
			}
		}
	})
	t.Run("existing keys skipped", func(t *testing.T) {
		if err := os.WriteFile(dst, []byte("[secrets]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		f := &fakeFnox{existing: map[string][]string{dst: {"S"}}}
		m, e, _ := newMigratorEnv(t, kc, f)
		e.existing = map[string][]string{miseDst: {"A"}}
		report, err := m.Files(ctx, Options{Files: []string{tmpl}})
		if err != nil {
			t.Fatal(err)
		}
		if report.Migrated != 1 || report.Skipped != 2 || len(f.writes) != 0 || len(e.sets) != 1 || e.sets[0].key != "B" {
			t.Errorf("report = %+v, fnox = %+v, mise = %+v; want only B written", report, f.writes, e.sets)
		}
	})
}

func TestFilesMiseFailureCounted(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, ".env.dotty")
	if err := os.WriteFile(tmpl, []byte("A=1\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, e, errOut := newMigratorEnv(t, &fakeKeychain{}, &fakeFnox{})
	e.failKeys = []string{"A"}
	report, err := m.Files(context.Background(), Options{Files: []string{tmpl}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 || report.Failed != 1 || !strings.Contains(errOut.String(), "fake mise failure for A") {
		t.Errorf("report = %+v, stderr = %q; want A failed and warned, B migrated", report, errOut.String())
	}
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
