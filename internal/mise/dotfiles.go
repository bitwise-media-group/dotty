// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotfileTargets returns configured targets, including long-form tables mise
// may write after a user edits a declaration.
func (c *Client) DotfileTargets() (map[string]bool, error) {
	fragments, err := Fragments(c.Dir)
	if err != nil {
		return nil, err
	}
	files := append([]string{ConfigPath(c.Dir)}, fragments...)
	targets := make(map[string]bool)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		table := ""
		for line := range strings.Lines(string(data)) {
			trimmed := strings.TrimSpace(line)
			if header, ok := tableHeader(trimmed); ok {
				table = header
				if target, ok := strings.CutPrefix(header, "dotfiles."); ok {
					targets[target] = true
				}
			} else if table == "dotfiles" {
				if target, ok := entryKey(trimmed); ok {
					targets[target] = true
				}
			}
		}
	}
	return targets, nil
}

// RegisterDotfiles adds repository deployments without replacing user-edited
// entries, and preserves the config's permissions and unrelated content.
func (c *Client) RegisterDotfiles(lines []string) error {
	path := ConfigPath(c.Dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	data, n := MergeEntries(data, "dotfiles", lines, "dotty: registered home tree")
	if n == 0 {
		return nil
	}
	return writeConfig(path, data)
}

// DotfilesRoot sets the capture root for new files to the repository's home
// tree. A home-relative root is portable and gives mise an absolute source
// after expansion, so newly captured symlinks resolve from any directory.
func (c *Client) DotfilesRoot(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve dotfiles root: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if rel, ok := strings.CutPrefix(root, home+string(filepath.Separator)); ok {
		root = "~/" + filepath.ToSlash(rel)
	}
	path := ConfigPath(c.Dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	data, _ = MergeEntries(data, "settings.dotfiles", []string{fmt.Sprintf("root = %q", root)},
		"dotty: capture new dotfiles in the repository")
	return writeConfig(path, data)
}
