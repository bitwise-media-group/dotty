// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"os"
	"path/filepath"
	"testing"
)

// copyTree builds a Tree whose "hooks" entry is copy-deployed, with one
// executable hook and one plain file in the source.
func copyTree(t *testing.T, dir string) Tree {
	t.Helper()
	source, target := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	write(t, filepath.Join(source, "hooks", "policy"), "#!/bin/sh\n")
	write(t, filepath.Join(source, "hooks", "policy.json"), "{}")
	if err := os.Chmod(filepath.Join(source, "hooks", "policy"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Tree{Source: source, Target: target, Copy: []string{"hooks"}}
}

// assertRealFile fails unless path is a regular file (not a symlink) holding
// content with the wanted permission bits.
func assertRealFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s mode = %v, want a regular file", path, info.Mode())
	}
	if info.Mode().Perm() != perm {
		t.Fatalf("%s perm = %o, want %o", path, info.Mode().Perm(), perm)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != content {
		t.Fatalf("%s content = %q, %v; want %q", path, got, err, content)
	}
}

func TestApplyCopyDirCreatesRealCopies(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)

	rep, err := Apply(tree, resolveAll(ResFail), "")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	hooks := filepath.Join(tree.Target, "hooks")
	if len(rep.Copied) != 1 || rep.Copied[0] != hooks {
		t.Fatalf("Copied = %v, want [%s]", rep.Copied, hooks)
	}
	if info, err := os.Lstat(hooks); err != nil || !info.IsDir() {
		t.Fatalf("hooks site = %v, %v; want a real directory", info, err)
	}
	assertRealFile(t, filepath.Join(hooks, "policy"), "#!/bin/sh\n", 0o755)
	assertRealFile(t, filepath.Join(hooks, "policy.json"), "{}", 0o644)
}

func TestApplyCopyDirIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)

	if _, err := Apply(tree, resolveAll(ResFail), ""); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	rep, err := Apply(tree, resolveAll(ResFail), "")
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if len(rep.Copied) != 0 {
		t.Fatalf("second Apply recopied: %v", rep.Copied)
	}
	if rep.OK != 1 {
		t.Fatalf("OK = %d, want 1", rep.OK)
	}
}

// TestApplyCopyDirReplacesSymlink pins the migration this mode exists for:
// a site linked by an older deployment becomes a real directory of copies.
func TestApplyCopyDirReplacesSymlink(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)
	if err := os.MkdirAll(tree.Target, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(tree.Target, "hooks")
	if err := os.Symlink(filepath.Join(tree.Source, "hooks"), hooks); err != nil {
		t.Fatal(err)
	}

	rep, err := Apply(tree, resolveAll(ResFail), "")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(rep.Copied) != 1 {
		t.Fatalf("Copied = %v, want the hooks site", rep.Copied)
	}
	if info, err := os.Lstat(hooks); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("hooks site still a symlink: %v, %v", info, err)
	}
	assertRealFile(t, filepath.Join(hooks, "policy"), "#!/bin/sh\n", 0o755)
	// The repository directory the symlink named survives.
	if _, err := os.Stat(filepath.Join(tree.Source, "hooks", "policy")); err != nil {
		t.Fatalf("source hooks disturbed: %v", err)
	}
}

func TestApplyCopyDirResyncsDrift(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)
	hooks := filepath.Join(tree.Target, "hooks")
	write(t, filepath.Join(hooks, "policy"), "stale")
	write(t, filepath.Join(hooks, "policy.json"), "{}")
	write(t, filepath.Join(hooks, "orphan"), "no longer in the repository")
	if err := os.Chmod(filepath.Join(hooks, "policy.json"), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Apply(tree, resolveAll(ResFail), "")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(rep.Copied) != 1 {
		t.Fatalf("Copied = %v, want the hooks site", rep.Copied)
	}
	assertRealFile(t, filepath.Join(hooks, "policy"), "#!/bin/sh\n", 0o755)
	assertRealFile(t, filepath.Join(hooks, "policy.json"), "{}", 0o644)
	if _, err := os.Lstat(filepath.Join(hooks, "orphan")); err == nil {
		t.Fatal("orphan survived the mirror sync")
	}
}

// TestApplyCopyDirConflictNeverAdopts pins that a regular file occupying a
// copy site is backed up even under an adopt-everything resolver — moving it
// over the source would destroy the repository's directory.
func TestApplyCopyDirConflictNeverAdopts(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)
	backup := filepath.Join(dir, "backup")
	hooks := filepath.Join(tree.Target, "hooks")
	write(t, hooks+".tmp", "user file")
	if err := os.Rename(hooks+".tmp", hooks); err != nil {
		t.Fatal(err)
	}

	rep, err := Apply(tree, resolveAll(ResAdopt), backup)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(rep.Backed) != 1 || len(rep.Adopted) != 0 {
		t.Fatalf("Backed = %v, Adopted = %v; want the file backed up", rep.Backed, rep.Adopted)
	}
	mirror := filepath.Join(backup, hooks)
	if got, err := os.ReadFile(mirror); err != nil || string(got) != "user file" {
		t.Fatalf("backup mirror = %q, %v", got, err)
	}
	assertRealFile(t, filepath.Join(hooks, "policy"), "#!/bin/sh\n", 0o755)
	if _, err := os.Stat(filepath.Join(tree.Source, "hooks", "policy")); err != nil {
		t.Fatalf("source hooks disturbed: %v", err)
	}
}

func TestPlanReportsCopyStates(t *testing.T) {
	dir := t.TempDir()
	tree := copyTree(t, dir)

	actions, err := Plan(tree)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 || !actions[0].Copy || actions[0].State != StateLink {
		t.Fatalf("fresh plan = %+v, want one missing copy action", actions)
	}

	if _, err := Apply(tree, resolveAll(ResFail), ""); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	actions, err = Plan(tree)
	if err != nil {
		t.Fatalf("Plan after Apply: %v", err)
	}
	if len(actions) != 1 || actions[0].State != StateOK {
		t.Fatalf("synced plan = %+v, want one ok copy action", actions)
	}

	write(t, filepath.Join(tree.Target, "hooks", "policy.json"), "{drifted}")
	actions, err = Plan(tree)
	if err != nil {
		t.Fatalf("Plan after drift: %v", err)
	}
	if len(actions) != 1 || actions[0].State != StateRelink {
		t.Fatalf("drifted plan = %+v, want one stale copy action", actions)
	}
}
