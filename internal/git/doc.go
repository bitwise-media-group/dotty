// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package git drives git from dotty for commit re-signing. Resign rebases a
// range of commits to re-create and sign each one through git's configured
// signing program (which is dotty itself); with author reset it also rewrites
// each commit's author to the current user.name/user.email and updates any
// trailer that named the original author. Everything shells out to git via a
// small Runner interface so tests substitute fakes.
package git
