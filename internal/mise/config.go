// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// The TOML tables dotty reads and edits. Everything else in a config file
// passes through untouched.
const (
	TableTools    = "tools"
	TablePackages = "bootstrap.packages"
	TableTaps     = "bootstrap.brew.taps"
)

// DefaultConfig is the user-owned config.toml a profile starts with: the
// lockfile setting, cask adoption, and the empty tables `mise use -g` and
// `mise bootstrap packages use -g` write into. It mirrors the scaffold's
// template/mise/config.toml; a test keeps the two identical. `locked` is
// deliberately not set: with a lockfile present it makes `mise use` refuse
// new tools and `mise install` refuse the backends that carry no download
// URL (pipx, npm, the core runtimes). Reproducibility comes from the
// lockfile itself — mise resolves "latest" to the locked version when the
// file has one — and from Sync locking before it installs.
const DefaultConfig = `# User-owned: ` + "`dotty packages add`" + ` / ` + "`mise use -g`" + ` write here.
# Component packages live in conf.d/ and are managed by ` + "`dotty init`" + `.
[settings]
lockfile = true

[bootstrap.brew]
adopt = true

[tools]

[bootstrap.packages]
`

// Dir returns the mise directory inside a profile directory.
func Dir(profileDir string) string { return filepath.Join(profileDir, "mise") }

// ConfigPath returns the user-owned config.toml inside a mise directory.
func ConfigPath(dir string) string { return filepath.Join(dir, "config.toml") }

// ConfDDir returns the conf.d directory of component fragments inside a
// mise directory.
func ConfDDir(dir string) string { return filepath.Join(dir, "conf.d") }

// LockPath returns the lockfile inside a mise directory.
func LockPath(dir string) string { return filepath.Join(dir, "mise.lock") }

// Entry is one declared tool or bootstrap package: the table it sits in, the
// file declaring it, its id, and the exact line (or subtable header) that
// declares it.
type Entry struct {
	Table string
	File  string
	ID    string
	Line  string
}

// Declared returns every tool and bootstrap package the profile's mise
// directory declares, keyed by id, from config.toml and the conf.d
// fragments in name order. A later declaration of the same id wins, the
// way mise merges the files. A missing directory declares nothing.
func Declared(dir string) (map[string]Entry, error) {
	declared := make(map[string]Entry)
	fragments, err := Fragments(dir)
	if err != nil {
		return nil, err
	}
	files := append([]string{ConfigPath(dir)}, fragments...)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		for _, e := range scanEntries(data, file) {
			if e.Table == TableTools || e.Table == TablePackages {
				declared[e.ID] = e
			}
		}
	}
	return declared, nil
}

// Fragments returns the conf.d fragment paths inside a mise directory,
// sorted; a missing directory has none.
func Fragments(dir string) ([]string, error) {
	fragments, err := filepath.Glob(filepath.Join(ConfDDir(dir), "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", ConfDDir(dir), err)
	}
	slices.Sort(fragments)
	return fragments, nil
}

// scanEntries walks one TOML document line by line and returns the entries
// of every table, keyed the way mise writes them: one `"<id>" = <value>`
// line per entry (inline tables included), or a `[<table>."<id>"]` subtable
// header for entries spelled out long-hand. It is deliberately not a TOML
// parser — dotty never merges documents, only finds and drops lines — so
// multi-line values are not understood; mise itself never writes them.
func scanEntries(data []byte, file string) []Entry {
	var entries []Entry
	table := ""
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\n"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if header, ok := tableHeader(trimmed); ok {
			table = header
			if parent, id, ok := splitSubtable(header); ok {
				entries = append(entries, Entry{Table: parent, File: file, ID: id, Line: trimmed})
			}
			continue
		}
		key, ok := entryKey(trimmed)
		if !ok || table == "" {
			continue
		}
		entries = append(entries, Entry{Table: table, File: file, ID: key, Line: trimmed})
	}
	return entries
}

// tableHeader parses a `[a.b."c"]` header into its dotted path with the
// quoting of each segment removed.
func tableHeader(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return "", false
	}
	end := strings.Index(trimmed, "]")
	if end < 0 {
		return "", false
	}
	inner := strings.TrimSpace(trimmed[1:end])
	var parts []string
	for inner != "" {
		key, rest, ok := readKey(inner)
		if !ok {
			return "", false
		}
		parts = append(parts, key)
		inner = strings.TrimSpace(rest)
		if inner == "" {
			break
		}
		if !strings.HasPrefix(inner, ".") {
			return "", false
		}
		inner = strings.TrimSpace(inner[1:])
	}
	return strings.Join(parts, "."), len(parts) > 0
}

// splitSubtable splits a subtable header path into the entry table and the
// entry id it declares: `tools.aqua:x/y` → (tools, aqua:x/y),
// `bootstrap.packages.brew:git` → (bootstrap.packages, brew:git).
func splitSubtable(path string) (parent, id string, ok bool) {
	for _, table := range []string{TableTools, TablePackages, TableTaps} {
		if rest, found := strings.CutPrefix(path, table+"."); found && rest != "" {
			return table, rest, true
		}
	}
	return "", "", false
}

// entryKey returns the key of a `key = value` line.
func entryKey(trimmed string) (string, bool) {
	key, rest, ok := readKey(trimmed)
	if !ok {
		return "", false
	}
	if !strings.HasPrefix(strings.TrimSpace(rest), "=") {
		return "", false
	}
	return key, true
}

// readKey reads one TOML key — quoted ("…" or '…') or bare — from the start
// of s and returns the remainder.
func readKey(s string) (key, rest string, ok bool) {
	if s == "" {
		return "", "", false
	}
	if quote := s[0]; quote == '"' || quote == '\'' {
		end := strings.IndexByte(s[1:], quote)
		if end < 0 {
			return "", "", false
		}
		return s[1 : 1+end], s[2+end:], true
	}
	end := strings.IndexFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '=' || r == '.' || r == ']'
	})
	if end < 0 {
		end = len(s)
	}
	if end == 0 {
		return "", "", false
	}
	return s[:end], s[end:], true
}

// RemoveEntry drops the declaration of id from the config file at path: the
// `"<id>" = …` line in whichever table declares it, or the whole
// `[<table>."<id>"]` subtable. Every other byte of the file — comments,
// blanks, other tables — is preserved. A missing id is not an error.
func RemoveEntry(path, id string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var out bytes.Buffer
	table := ""
	skipping := false // inside the subtable being removed
	changed := false
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\n"))
		if header, ok := tableHeader(trimmed); ok {
			table = header
			parent, sub, isSub := splitSubtable(header)
			skipping = isSub && sub == id && (parent == TableTools || parent == TablePackages)
			if skipping {
				changed = true
				continue
			}
		} else if skipping {
			continue
		} else if key, ok := entryKey(trimmed); ok && key == id && (table == TableTools || table == TablePackages) {
			changed = true
			continue
		}
		out.WriteString(line)
	}
	if !changed {
		return nil
	}
	return writeConfig(path, out.Bytes())
}

// MergeEntries appends lines — `"<id>" = <value>` declarations — to table
// in existing, under a `# <header>` comment, and returns the new document
// with the number of lines it added. The lines land at the end of the
// table's section when the document has one, else in a new table at the
// end of the document. Lines whose id the document already declares in
// that table are dropped, so a merge is idempotent; nothing new returns
// existing untouched.
func MergeEntries(existing []byte, table string, lines []string, header string) ([]byte, int) {
	present := make(map[string]bool)
	for _, e := range scanEntries(existing, "") {
		if e.Table == table {
			present[e.ID] = true
		}
	}
	var added []string
	for _, line := range lines {
		key, ok := entryKey(strings.TrimSpace(line))
		if !ok || present[key] {
			continue
		}
		present[key] = true
		added = append(added, strings.TrimSpace(line))
	}
	if len(added) == 0 {
		return existing, 0
	}

	var block bytes.Buffer
	block.WriteString("# ")
	block.WriteString(header)
	block.WriteByte('\n')
	for _, line := range added {
		block.WriteString(line)
		block.WriteByte('\n')
	}

	end, found := tableEnd(existing, table)
	if !found {
		out := bytes.TrimRight(existing, "\n")
		if len(out) > 0 {
			out = append(out, "\n\n"...)
		}
		out = append(out, "["+table+"]\n"...)
		return append(out, block.Bytes()...), len(added)
	}
	body := bytes.TrimRight(existing[:end], "\n")
	out := append([]byte{}, body...)
	out = append(out, '\n')
	out = append(out, block.Bytes()...)
	if rest := existing[end:]; len(rest) > 0 {
		out = append(out, '\n')
		out = append(out, rest...)
	}
	return out, len(added)
}

// tableEnd locates the end of table's section in data: the byte offset
// where the next header (or the document) begins.
func tableEnd(data []byte, table string) (end int, found bool) {
	offset := 0
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\n"))
		if header, ok := tableHeader(trimmed); ok {
			if found {
				return offset, true
			}
			found = header == table
		}
		offset += len(line)
	}
	if found {
		return len(data), true
	}
	return 0, false
}

// writeConfig writes data to path atomically, keeping the file's mode.
func writeConfig(path string, data []byte) error {
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := cli.AtomicWriteFile(path, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// EnsureConfig creates the user-owned config.toml from DefaultConfig when
// the mise directory has none — a hand-made profile, or one from before
// packages lived in mise. An existing file is left alone.
func EnsureConfig(dir string) error {
	path := ConfigPath(dir)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if err := cli.EnsureDir(dir, 0o755); err != nil {
		return err
	}
	return writeConfig(path, []byte(DefaultConfig))
}

// Merge headers: the comment each merge source writes its entries under.
const (
	HeaderBrewfile = "imported from Brewfile"
	HeaderImported = "installed packages"
)

// MergeConversion appends the tools, packages, and taps of a Brewfile
// conversion that the profile does not already declare to its config.toml,
// under the HeaderBrewfile comment, and returns how many lines were added.
// The file is rewritten only when something is new.
func MergeConversion(dir string, conv Conversion) (int, error) {
	declared, err := Declared(dir)
	if err != nil {
		return 0, err
	}
	byTable := make(map[string][]string)
	for _, c := range conv.Tools {
		if _, ok := declared[c.ID]; !ok {
			byTable[TableTools] = append(byTable[TableTools], c.Line)
		}
	}
	for _, c := range conv.Packages {
		if _, ok := declared[c.ID]; !ok {
			byTable[TablePackages] = append(byTable[TablePackages], c.Line)
		}
	}
	for _, tap := range slices.Sorted(maps.Keys(conv.Taps)) {
		byTable[TableTaps] = append(byTable[TableTaps], TapLine(tap, conv.Taps[tap]))
	}
	return mergeIntoConfig(dir, byTable, HeaderBrewfile)
}

// MergeImported appends the entries an import produced (see ImportToScratch)
// that the profile does not already declare to its config.toml, under the
// HeaderImported comment, and returns how many lines were added.
func MergeImported(dir string, entries []Entry) (int, error) {
	declared, err := Declared(dir)
	if err != nil {
		return 0, err
	}
	byTable := make(map[string][]string)
	for _, e := range entries {
		if _, ok := declared[e.ID]; ok {
			continue
		}
		byTable[e.Table] = append(byTable[e.Table], e.Line)
	}
	return mergeIntoConfig(dir, byTable, HeaderImported)
}

// mergeIntoConfig appends the per-table lines to the profile's config.toml
// under header and returns how many lines were new. The file is rewritten
// only when something changed.
func mergeIntoConfig(dir string, byTable map[string][]string, header string) (int, error) {
	path := ConfigPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	added := 0
	for _, table := range []string{TableTools, TablePackages, TableTaps} {
		lines := byTable[table]
		if len(lines) == 0 {
			continue
		}
		var n int
		data, n = MergeEntries(data, table, lines, header)
		added += n
	}
	if added == 0 {
		return 0, nil
	}
	return added, writeConfig(path, data)
}
