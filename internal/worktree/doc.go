// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package worktree holds the pure logic of the agent-worktree lifecycle —
// root resolution, name sanitization, identifier derivation, and hook-JSON
// parsing — with no I/O, so it is exhaustively unit-testable. The git and
// tmux side effects live in internal/git and the git worktree commands that
// wrap it.
package worktree
