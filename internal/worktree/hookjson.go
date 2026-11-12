// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package worktree

import "encoding/json"

// ParseAddName extracts ".name" from WorktreeCreate hook JSON on stdin.
// Invalid or empty input yields "".
func ParseAddName(data []byte) string {
	var h struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(data, &h)
	return h.Name
}

// ParseEndPath extracts ".worktree_path" from WorktreeRemove hook JSON on
// stdin. Invalid or empty input yields "".
func ParseEndPath(data []byte) string {
	var h struct {
		WorktreePath string `json:"worktree_path"`
	}
	_ = json.Unmarshal(data, &h)
	return h.WorktreePath
}
