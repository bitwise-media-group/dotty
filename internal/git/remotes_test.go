// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestPublishBranch(t *testing.T) {
	m := newMem()
	if err := PublishBranch(context.Background(), m, "feat-1"); err != nil {
		t.Fatalf("PublishBranch() error: %v", err)
	}
	want := []string{"push", "-u", "origin", "feat-1"}
	if len(m.pushes) != 1 || !slices.Equal(m.pushes[0], want) {
		t.Fatalf("pushes = %v, want [%v]", m.pushes, want)
	}
}

func TestPublishBranchNoPushRemote(t *testing.T) {
	m := newMem()
	m.remotes = "upstream\n"
	if err := PublishBranch(context.Background(), m, "feat-1"); err != nil {
		t.Fatalf("PublishBranch() with no origin: %v", err)
	}
	if len(m.pushes) != 0 {
		t.Fatalf("pushes = %v, want none", m.pushes)
	}
}

func TestPublishBranchPushFailure(t *testing.T) {
	m := newMem()
	m.pushErr = errors.New("remote unreachable")
	err := PublishBranch(context.Background(), m, "feat-1")
	if err == nil || !errors.Is(err, m.pushErr) {
		t.Fatalf("PublishBranch() error = %v, want wrapped %v", err, m.pushErr)
	}
}
