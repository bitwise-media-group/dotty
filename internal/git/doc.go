// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package git drives git from dotty: commit re-signing and
// signature-preserving stacked branches for fork workflows.
//
// Resign rebases a range of commits to re-create and sign each one through
// git's configured signing program (which is dotty itself); with author reset
// it also rewrites each commit's author to the current user.name/user.email
// and updates any trailer that named the original author.
//
// The stack workflow manages local lineage, trunk-based pull requests (all
// targeting upstream/main), stack navigation, and sync (cleanup merged
// layers, refresh PR body maps, rebase+resign when diverged). Lineage is
// stored in the repository's local git config under the dotty.stack.* and
// dotty.branch.* namespaces. Landing is always via the org's ff-merge bot;
// this package never rewrites main.
//
// Everything shells out to git via a small Runner interface so tests
// substitute fakes.
package git
