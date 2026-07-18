// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// mergeFixture saves a three-layer stack (a → b → c, bottom to tip) into a
// fresh memRunner and returns both.
func mergeFixture(t *testing.T) (*memRunner, Stack) {
	t.Helper()
	m := newMem()
	s := Stack{ID: "beef0000", Layers: []Layer{
		{Branch: "feat-a", PR: 11},
		{Branch: "feat-b"},
		{Branch: "feat-c", PR: 13},
	}}
	if err := SaveStack(context.Background(), m, s); err != nil {
		t.Fatal(err)
	}
	return m, s
}

func TestMergeParentsCollapsesLayers(t *testing.T) {
	m, s := mergeFixture(t)
	ctx := context.Background()

	got, merged, err := MergeParents(ctx, m, s, "feat-c", 2, CleanupConfig{Enabled: true})
	if err != nil {
		t.Fatalf("MergeParents: %v", err)
	}
	if !slices.Equal(merged, []string{"feat-a", "feat-b"}) {
		t.Fatalf("merged = %v", merged)
	}
	if len(got.Layers) != 1 || got.Layers[0].Branch != "feat-c" {
		t.Fatalf("stack after merge = %+v", got.Layers)
	}
	if !slices.Equal(m.deleted, []string{"feat-a", "feat-b"}) {
		t.Fatalf("deleted branches = %v", m.deleted)
	}
	for _, key := range []string{branchStackKey("feat-a"), branchPRKey("feat-a"), branchStackKey("feat-b")} {
		if m.config[key] != "" {
			t.Errorf("lineage key %s survived the merge", key)
		}
	}
	if m.config[stackLayersKey("beef0000")] != "feat-c" {
		t.Errorf("layers key = %q", m.config[stackLayersKey("beef0000")])
	}
}

func TestMergeParentsSingleParentDefault(t *testing.T) {
	m, s := mergeFixture(t)

	got, merged, err := MergeParents(context.Background(), m, s, "feat-c", 1, CleanupConfig{Enabled: true})
	if err != nil {
		t.Fatalf("MergeParents: %v", err)
	}
	if !slices.Equal(merged, []string{"feat-b"}) {
		t.Fatalf("merged = %v", merged)
	}
	if got.IndexOf("feat-a") != 0 || got.IndexOf("feat-c") != 1 {
		t.Fatalf("stack after merge = %+v", got.Layers)
	}
}

func TestMergeParentsRejectsExcessCount(t *testing.T) {
	m, s := mergeFixture(t)

	_, _, err := MergeParents(context.Background(), m, s, "feat-c", 3, CleanupConfig{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "2 parent layer(s)") {
		t.Fatalf("err = %v, want the parent-count refusal", err)
	}
	if len(m.deleted) != 0 {
		t.Fatalf("branches deleted despite refusal: %v", m.deleted)
	}
}

func TestMergeParentsRejectsOutOfSyncRange(t *testing.T) {
	m, s := mergeFixture(t)
	m.notIn["feat-b|feat-c"] = 2 // feat-b was amended; feat-c lacks its commits

	_, _, err := MergeParents(context.Background(), m, s, "feat-c", 2, CleanupConfig{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "not in sync") {
		t.Fatalf("err = %v, want the out-of-sync refusal", err)
	}
	if len(m.deleted) != 0 {
		t.Fatalf("branches deleted despite refusal: %v", m.deleted)
	}
	if m.config[stackLayersKey("beef0000")] != "feat-a,feat-b,feat-c" {
		t.Fatalf("stack metadata changed despite refusal: %q", m.config[stackLayersKey("beef0000")])
	}
}

func TestMergeParentsHonoursCleanupConfig(t *testing.T) {
	m, s := mergeFixture(t)

	got, _, err := MergeParents(context.Background(), m, s, "feat-c", 1, CleanupConfig{Enabled: false})
	if err != nil {
		t.Fatalf("MergeParents: %v", err)
	}
	if len(m.deleted) != 0 {
		t.Fatalf("cleanup disabled but branches deleted: %v", m.deleted)
	}
	if got.IndexOf("feat-b") >= 0 {
		t.Fatal("layer stayed in the stack metadata")
	}
}
