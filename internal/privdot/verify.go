// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Finding is one hygiene problem verify spotted in the repository.
type Finding struct {
	// Path is repository-relative.
	Path    string
	Problem string
}

// sensitiveBases are filenames that are secrets by convention; carrying one
// unencrypted in a private repository is almost always a mistake.
var sensitiveBases = []string{"known_hosts", "authorized_keys", "allowed_signers"}

// VerifyPaths checks repository-relative paths for plaintext accidents: a
// plaintext copy sitting beside its ciphertext, or a sensitive-looking file
// that never got encrypted. It reads names and the tree only — never
// content, so it stays safe for a pre-commit hook.
func VerifyPaths(repo string, rels []string) []Finding {
	var findings []Finding
	for _, rel := range rels {
		base := filepath.Base(rel)
		if base == gitkeepName || strings.HasSuffix(base, CipherExt) {
			continue
		}
		if !strings.Contains(rel, string(filepath.Separator)+"home"+string(filepath.Separator)) {
			continue // repository furniture: markers, age/, hooks
		}
		if _, err := os.Stat(filepath.Join(repo, rel+CipherExt)); err == nil {
			findings = append(findings, Finding{Path: rel,
				Problem: "plaintext sibling of an encrypted file; remove it (the ciphertext is authoritative)"})
			continue
		}
		if looksSensitive(base) {
			findings = append(findings, Finding{Path: rel,
				Problem: "looks sensitive but is not encrypted; adopt it with dotty private encrypt"})
		}
	}
	return findings
}

// VerifyRepo runs VerifyPaths over every file the repository tracks on disk,
// plus per-profile shape checks: ciphertext with no recipients file, and
// recipients with no identity stubs (or the reverse) — each a sign an
// enrollment never finished.
func VerifyRepo(repo string) ([]Finding, error) {
	profiles, err := ListProfiles(repo)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, p := range profiles {
		files, err := ListFiles(repo, p)
		if err != nil {
			return nil, err
		}
		var rels []string
		encrypted := false
		for _, f := range files {
			rel, err := filepath.Rel(repo, f.RepoPath)
			if err != nil {
				continue
			}
			rels = append(rels, rel)
			encrypted = encrypted || f.Encrypted
		}
		findings = append(findings, VerifyPaths(repo, rels)...)

		recipients := RecipientsPath(repo, p)
		_, recErr := os.Stat(recipients)
		ids, err := Identities(repo, p)
		if err != nil {
			return nil, err
		}
		relRecipients, _ := filepath.Rel(repo, recipients)
		if encrypted && recErr != nil {
			findings = append(findings, Finding{Path: relRecipients,
				Problem: "profile has ciphertext but no recipients file; run dotty private enroll"})
		}
		if recErr == nil && len(ids) == 0 {
			findings = append(findings, Finding{Path: relRecipients,
				Problem: "recipients without identity stubs; decryption here is impossible — rerun dotty private enroll"})
		}
		if recErr != nil && len(ids) > 0 {
			findings = append(findings, Finding{Path: relRecipients,
				Problem: "identity stubs without a recipients file; encryption is impossible — rerun dotty private enroll"})
		}
	}
	return findings, nil
}

// looksSensitive reports whether a filename names a secret by convention.
func looksSensitive(base string) bool {
	if slices.Contains(sensitiveBases, base) {
		return true
	}
	if strings.HasPrefix(base, "id_") && !strings.HasSuffix(base, ".pub") {
		return true
	}
	if strings.Contains(base, "history") {
		return true
	}
	switch filepath.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx":
		return true
	}
	return false
}
