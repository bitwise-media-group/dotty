// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const richConfig = `# header comment
[settings]
lockfile = true

[tools]
"aqua:sharkdp/fd" = "latest" # trailing comment
node = "lts"
"github:docker/compose" = { version = "latest", postinstall = "ln -sf x y" }

[tools."aqua:anchore/quill"]
version = "latest"

[bootstrap.brew]
adopt = true

[bootstrap.packages]
"brew:git" = "latest"
"brew-cask:ghostty" = { version = "latest", os = "macos" }

[bootstrap.packages."mas:497799835"]
version = "latest"
`

// TestDeclared pins the scanner: quoted and bare keys, inline tables,
// subtable declarations, and the config.toml-then-fragments precedence
// with each entry attributed to its file.
func TestDeclared(t *testing.T) {
	dir := profileDir(t, richConfig, map[string]string{
		"core.toml":       "[tools]\n\"aqua:jqlang/jq\" = \"latest\"\nnode = \"latest\"\n",
		"addon-tmux.toml": "[bootstrap.packages]\n\"brew:tmux\" = \"latest\"\n",
	})
	declared, err := Declared(dir)
	if err != nil {
		t.Fatalf("Declared: %v", err)
	}
	want := map[string]Entry{
		"aqua:sharkdp/fd":       {Table: TableTools, File: ConfigPath(dir)},
		"github:docker/compose": {Table: TableTools, File: ConfigPath(dir)},
		"aqua:anchore/quill":    {Table: TableTools, File: ConfigPath(dir)},
		"brew:git":              {Table: TablePackages, File: ConfigPath(dir)},
		"brew-cask:ghostty":     {Table: TablePackages, File: ConfigPath(dir)},
		"mas:497799835":         {Table: TablePackages, File: ConfigPath(dir)},
		"aqua:jqlang/jq":        {Table: TableTools, File: filepath.Join(ConfDDir(dir), "core.toml")},
		"node":                  {Table: TableTools, File: filepath.Join(ConfDDir(dir), "core.toml")}, // fragment wins
		"brew:tmux":             {Table: TablePackages, File: filepath.Join(ConfDDir(dir), "addon-tmux.toml")},
	}
	for id, w := range want {
		got, ok := declared[id]
		if !ok {
			t.Errorf("%s not declared", id)
			continue
		}
		if got.Table != w.Table || got.File != w.File || got.ID != id {
			t.Errorf("%s = %+v, want table %s in %s", id, got, w.Table, w.File)
		}
	}
	if len(declared) != len(want) {
		t.Errorf("declared %d entries, want %d: %v", len(declared), len(want), declared)
	}
	if got, err := Declared(filepath.Join(t.TempDir(), "missing")); err != nil || len(got) != 0 {
		t.Errorf("missing dir: %v, %v; want none", got, err)
	}
}

// TestRemoveEntry pins the line drop: only the named entry goes, comments,
// blanks, the other tables, inline tables, and subtables survive — and a
// subtable removal takes its body along.
func TestRemoveEntry(t *testing.T) {
	cases := []struct {
		name  string
		id    string
		gone  []string
		keeps []string
	}{
		{
			name: "inline-table package", id: "brew-cask:ghostty",
			gone:  []string{"ghostty"},
			keeps: []string{`"brew:git" = "latest"`, `[bootstrap.packages."mas:497799835"]`, "# header comment"},
		},
		{
			name: "bare-key tool", id: "node",
			gone:  []string{"node = "},
			keeps: []string{`"aqua:sharkdp/fd" = "latest" # trailing comment`, "postinstall"},
		},
		{
			name: "subtable tool", id: "aqua:anchore/quill",
			gone:  []string{"quill"},
			keeps: []string{"[bootstrap.brew]\nadopt = true", "[bootstrap.packages]\n"},
		},
		{
			name: "subtable package", id: "mas:497799835",
			gone:  []string{"mas:"},
			keeps: []string{`"brew-cask:ghostty" = { version = "latest", os = "macos" }`},
		},
		{
			name: "missing id is a no-op", id: "brew:nope",
			keeps: []string{richConfig},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(richConfig), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := RemoveEntry(path, c.id); err != nil {
				t.Fatalf("RemoveEntry: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range c.gone {
				if strings.Contains(string(got), g) {
					t.Errorf("%q survived:\n%s", g, got)
				}
			}
			for _, k := range c.keeps {
				if !strings.Contains(string(got), k) {
					t.Errorf("%q lost:\n%s", k, got)
				}
			}
			// "version = ..." belongs only to the subtables; removing one
			// must not strand the other's body under the wrong header.
			if c.id == "aqua:anchore/quill" && strings.Count(string(got), "\nversion = \"latest\"") != 1 {
				t.Errorf("subtable body handling wrong:\n%s", got)
			}
		})
	}
}

// TestMergeEntries pins the append: under the table's section when present
// (before the next table), at EOF in a new table otherwise, under the
// header comment, dropping ids the table already declares, and leaving the
// document untouched when nothing is new.
func TestMergeEntries(t *testing.T) {
	existing := []byte("[settings]\nlockfile = true\n\n[tools]\n\"aqua:sharkdp/fd\" = \"latest\"\n\n" +
		"[bootstrap.packages]\n\"brew:git\" = \"latest\"\n")

	t.Run("existing table", func(t *testing.T) {
		merged, n := MergeEntries(existing, TableTools,
			[]string{`"aqua:sharkdp/fd" = "latest"`, `"aqua:jqlang/jq" = "latest"`, `go = "latest"`}, "imported")
		got := string(merged)
		if n != 2 {
			t.Errorf("added = %d, want 2", n)
		}
		want := "[settings]\nlockfile = true\n\n[tools]\n\"aqua:sharkdp/fd\" = \"latest\"\n" +
			"# imported\n\"aqua:jqlang/jq\" = \"latest\"\ngo = \"latest\"\n\n" +
			"[bootstrap.packages]\n\"brew:git\" = \"latest\"\n"
		if got != want {
			t.Errorf("merged =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("last table", func(t *testing.T) {
		merged, _ := MergeEntries(existing, TablePackages, []string{`"brew:wget" = "latest"`}, "installed packages")
		got := string(merged)
		want := "[bootstrap.packages]\n\"brew:git\" = \"latest\"\n# installed packages\n\"brew:wget\" = \"latest\"\n"
		if !strings.HasSuffix(got, want) {
			t.Errorf("merged =\n%s", got)
		}
	})

	t.Run("missing table", func(t *testing.T) {
		merged, _ := MergeEntries(existing, TableTaps, []string{`"acme/tap" = "https://x"`}, "imported")
		got := string(merged)
		want := "\"brew:git\" = \"latest\"\n\n[bootstrap.brew.taps]\n# imported\n\"acme/tap\" = \"https://x\"\n"
		if !strings.HasSuffix(got, want) {
			t.Errorf("merged =\n%s", got)
		}
	})

	t.Run("empty document", func(t *testing.T) {
		merged, _ := MergeEntries(nil, TableTools, []string{`go = "latest"`}, "x")
		got := string(merged)
		if got != "[tools]\n# x\ngo = \"latest\"\n" {
			t.Errorf("merged = %q", got)
		}
	})

	t.Run("nothing new", func(t *testing.T) {
		got, n := MergeEntries(existing, TableTools, []string{`"aqua:sharkdp/fd" = "latest"`}, "x")
		if string(got) != string(existing) || n != 0 {
			t.Errorf("document changed:\n%s", got)
		}
	})
}

func TestEnsureConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mise")
	if err := EnsureConfig(dir); err != nil {
		t.Fatalf("EnsureConfig: %v", err)
	}
	got, err := os.ReadFile(ConfigPath(dir))
	if err != nil || string(got) != DefaultConfig {
		t.Errorf("config = %q, %v; want the default", got, err)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfig(dir); err != nil {
		t.Fatalf("EnsureConfig again: %v", err)
	}
	if got, _ := os.ReadFile(ConfigPath(dir)); string(got) != "custom" {
		t.Errorf("existing config overwritten: %q", got)
	}
}

func TestTableHeader(t *testing.T) {
	cases := map[string]string{
		"[tools]":                             "tools",
		`[tools."aqua:x/y"]`:                  "tools.aqua:x/y",
		"[ bootstrap . packages ]":            "bootstrap.packages",
		`[bootstrap.packages.'brew:git'] # c`: "bootstrap.packages.brew:git",
	}
	for in, want := range cases {
		got, ok := tableHeader(in)
		if !ok || got != want {
			t.Errorf("tableHeader(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"[[tools]]", "x = 1", "[tools"} {
		if _, ok := tableHeader(bad); ok {
			t.Errorf("tableHeader(%q) accepted", bad)
		}
	}
	ids := FragmentToolIDs()
	if !slices.IsSorted(ids) || slices.Contains(ids, "go") || !slices.Contains(ids, "http:grok") {
		t.Errorf("FragmentToolIDs = %v", ids)
	}
}
