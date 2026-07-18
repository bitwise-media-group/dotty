// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"strconv"
	"strings"
	"testing"
)

func TestBuildAndRewritePRBody(t *testing.T) {
	map1 := "## Stack\n\n* [ ] #1 `a` — feat: a\n"
	body := BuildPRBody("deadbeef", map1, "Layer details here.")
	if !strings.Contains(body, "<!-- dotty-stack:v1 id=deadbeef -->") {
		t.Fatalf("missing open marker: %s", body)
	}
	if !strings.Contains(body, "Layer details here.") {
		t.Fatalf("missing description: %s", body)
	}
	if got := ExtractDescription(body); got != "Layer details here." {
		t.Fatalf("ExtractDescription = %q", got)
	}

	map2 := "## Stack\n\n* [x] #1 `a` — feat: a *(merged)*\n* [ ] #2 `b` — feat: b\n"
	updated := RewriteStackSection(body, "deadbeef", map2)
	if !strings.Contains(updated, "*(merged)*") {
		t.Fatalf("expected merged marker: %s", updated)
	}
	if got := ExtractDescription(updated); got != "Layer details here." {
		t.Fatalf("description clobbered: %q", got)
	}
}

func TestFormatStackMap(t *testing.T) {
	s := Stack{
		ID: "abc",
		Layers: []Layer{
			{Branch: "b1", PR: 10, TitleHint: "feat: one"},
			{Branch: "b2", PR: 11, TitleHint: "feat: two"},
		},
	}
	merged := map[string]bool{"b1": true}
	md := FormatStackMap(s, "b2", func(n int) string {
		return "https://example.com/pull/" + strconv.Itoa(n)
	}, merged)
	if !strings.Contains(md, "[x]") || !strings.Contains(md, "you are here") {
		t.Fatalf("map = %s", md)
	}
	if !strings.Contains(md, "[#11](https://example.com/pull/11) — feat: two") {
		t.Fatalf("missing PR link row: %s", md)
	}
	// The PR reference plus title identifies the row; branch names stay out.
	if strings.Contains(md, "b1") || strings.Contains(md, "b2") {
		t.Fatalf("branch name leaked into map: %s", md)
	}
}

func TestFormatStackMapWithoutPR(t *testing.T) {
	s := Stack{ID: "abc", Layers: []Layer{{Branch: "b1", TitleHint: "feat: one"}, {Branch: "b2"}}}
	md := FormatStackMap(s, "b1", func(int) string { return "" }, nil)
	if !strings.Contains(md, "* [ ] `b1` — feat: one") {
		t.Fatalf("unproposed layer should keep its branch name: %s", md)
	}
	if !strings.Contains(md, "* [ ] `b2`\n") {
		t.Fatalf("titleless layer should render the branch alone: %s", md)
	}
}

func TestHTTPBrowseURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"git@github.com:org/repo.git", "https://github.com/org/repo"},
		{"https://github.com/org/repo.git", "https://github.com/org/repo"},
		{"ssh://git@github.com/org/repo", "https://github.com/org/repo"},
	}
	for _, tt := range tests {
		got, err := HTTPBrowseURL(tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveProposeScope(t *testing.T) {
	s := Stack{Layers: []Layer{{Branch: "a"}, {Branch: "b"}, {Branch: "c"}}}
	through, err := ResolveProposeScope(s, "b", false)
	if err != nil || through != 1 {
		t.Fatalf("mid = %d, %v", through, err)
	}
	through, err = ResolveProposeScope(s, "b", true)
	if err != nil || through != 2 {
		t.Fatalf("all = %d, %v", through, err)
	}
}

func TestFormatStatusAlignment(t *testing.T) {
	s := Stack{ID: "abc", Layers: nil}
	rows := []LayerStatus{
		{Layer: Layer{Branch: "feat/a", TitleHint: "feat: short"}, Index: 0, Relation: RelFF},
		{
			Layer:    Layer{Branch: "feat/use-xdg-for-agents", TitleHint: "feat: long branch", PR: 12},
			Index:    1,
			Relation: RelFF,
			Current:  true,
		},
		{Layer: Layer{Branch: "feat/b", TitleHint: "feat: mid"}, Index: 2, Relation: RelDiverged},
	}
	var buf strings.Builder
	FormatStatus(&buf, s, Trunk{Remote: "upstream", Branch: "main"}, rows)
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 { // header + 3 rows
		t.Fatalf("lines = %d\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], "stack abc") || !strings.Contains(lines[0], "upstream/main") {
		t.Fatalf("header: %q", lines[0])
	}
	// Relation column: find "ff" / "diverged" start index should match across rows.
	relCols := make([]int, 0, 3)
	for _, line := range lines[1:] {
		// strip leading spaces then look for relation words after branch
		iFF := strings.Index(line, "\t") // may already be expanded to spaces
		_ = iFF
		for _, rel := range []string{"ff", "diverged", "merged", "identical"} {
			if i := strings.Index(line, " "+rel+" "); i >= 0 {
				relCols = append(relCols, i)
				break
			}
			// end of padding before relation at end-ish: search bare
			if i := indexAsWord(line, rel); i >= 0 {
				relCols = append(relCols, i)
				break
			}
		}
	}
	if len(relCols) != 3 {
		t.Fatalf("could not find relation columns:\n%s", got)
	}
	if relCols[0] != relCols[1] || relCols[1] != relCols[2] {
		t.Fatalf("relation column misaligned: %v\n%s", relCols, got)
	}
	if !strings.Contains(lines[2], "*") || !strings.Contains(lines[2], "PR#12") {
		t.Fatalf("current/PR row: %q", lines[2])
	}
}

func indexAsWord(line, word string) int {
	// Find word preceded by ≥2 spaces (column gap) so we don't match inside branch names.
	needle := "  " + word
	return strings.Index(line, needle)
}
