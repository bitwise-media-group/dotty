// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"testing"
)

func TestAnyStaleDetectsChildBehindParent(t *testing.T) {
	m := newMem()
	rows := []LayerStatus{
		{Layer: Layer{Branch: "feat-a"}, Relation: RelFF},
		{Layer: Layer{Branch: "feat-b"}, Relation: RelFF},
	}
	// memRunner answers --is-ancestor from the ancestors map; the pair is
	// absent, so feat-b does not contain feat-a's tip.
	if !AnyStale(context.Background(), m, rows) {
		t.Fatal("want stale when feat-b does not descend from feat-a")
	}
}

func TestAnyStaleFalseWhenChainIntact(t *testing.T) {
	m := newMem()
	m.ancestors["feat-a|feat-b"] = true
	m.ancestors["feat-b|feat-c"] = true
	rows := []LayerStatus{
		{Layer: Layer{Branch: "feat-a"}, Relation: RelFF},
		{Layer: Layer{Branch: "feat-b"}, Relation: RelFF},
		{Layer: Layer{Branch: "feat-c"}, Relation: RelFF},
	}
	if AnyStale(context.Background(), m, rows) {
		t.Fatal("want in-sync when every layer descends from the one below")
	}
}

func TestAnyStaleSkipsMergedLayers(t *testing.T) {
	m := newMem()
	// feat-b is merged: feat-c's parent in the open chain is feat-a.
	m.ancestors["feat-a|feat-c"] = true
	rows := []LayerStatus{
		{Layer: Layer{Branch: "feat-a"}, Relation: RelFF},
		{Layer: Layer{Branch: "feat-b"}, Relation: RelMerged},
		{Layer: Layer{Branch: "feat-c"}, Relation: RelFF},
	}
	if AnyStale(context.Background(), m, rows) {
		t.Fatal("merged layer must not break the open-chain ancestry check")
	}
}

func TestAnyStaleSingleLayer(t *testing.T) {
	m := newMem()
	rows := []LayerStatus{{Layer: Layer{Branch: "feat-a"}, Relation: RelFF}}
	if AnyStale(context.Background(), m, rows) {
		t.Fatal("a single open layer has no pair to be stale against")
	}
}
