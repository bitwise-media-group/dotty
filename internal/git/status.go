// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
)

// LayerStatus is one row of `stack status` output.
type LayerStatus struct {
	Layer
	Index    int
	Relation Relation
	Current  bool
}

// Status reports each layer versus trunk.
func Status(ctx context.Context, r Runner, s Stack, trunk Trunk, current string) []LayerStatus {
	out := make([]LayerStatus, 0, len(s.Layers))
	for i, l := range s.Layers {
		rel, err := ClassifyTip(ctx, r, trunk.Ref(), l.Branch)
		if err != nil {
			// Branch may have been deleted locally after merge.
			rel = RelMerged
		}
		out = append(out, LayerStatus{
			Layer:    l,
			Index:    i,
			Relation: rel,
			Current:  l.Branch == current,
		})
	}
	return out
}

// FormatStatus writes a human-readable stack status with column-aligned rows.
func FormatStatus(w io.Writer, s Stack, trunk Trunk, rows []LayerStatus) {
	_, _ = fmt.Fprintf(w, "stack %s · trunk %s\n", s.ID, trunk.Ref())
	// tabwriter keeps mark / index / branch / relation / PR / subject aligned
	// even when branch names vary widely in length. Mark is always one cell
	// ("*" or " ") so the index column never shifts for the current layer.
	tw := tabwriter.NewWriter(w, 0, 1, 2, ' ', 0)
	for _, row := range rows {
		mark := " "
		if row.Current {
			mark = "*"
		}
		pr := ""
		if row.PR > 0 {
			pr = fmt.Sprintf("PR#%d", row.PR)
		}
		_, _ = fmt.Fprintf(tw, "%s\t[%d]\t%s\t%s\t%s\t%s\n",
			mark, row.Index+1, row.Branch, row.Relation, pr, row.TitleHint)
	}
	_ = tw.Flush()
}

// AnyDiverged reports whether any open layer has diverged from trunk.
func AnyDiverged(rows []LayerStatus) bool {
	return slices.ContainsFunc(rows, func(r LayerStatus) bool { return r.Relation == RelDiverged })
}

// AnyStale reports whether any open layer no longer descends from the open
// layer below it — a lower branch gained commits after the upper branch was
// cut. Such a stack needs a restack even though no tip has diverged from
// trunk, which the tip-vs-trunk relations alone cannot see.
func AnyStale(ctx context.Context, r Runner, rows []LayerStatus) bool {
	parent := ""
	for _, row := range rows {
		if row.Relation == RelMerged {
			continue
		}
		if parent != "" {
			if ok, err := IsAncestor(ctx, r, parent, row.Branch); err == nil && !ok {
				return true
			}
		}
		parent = row.Branch
	}
	return false
}

// FormatStackMap builds the markdown stack section for PR bodies.
// currentBranch is the PR this body is for (gets "you are here").
// mergedByBranch maps branch → true when that layer is merged (or not open).
func FormatStackMap(s Stack, currentBranch string, prURL func(pr int) string, merged map[string]bool) string {
	var b strings.Builder
	b.WriteString("## Stack\n\n")
	for _, l := range s.Layers {
		check := "[ ]"
		suffix := ""
		if merged[l.Branch] {
			check = "[x]"
			suffix = " *(merged)*"
		}
		here := ""
		if l.Branch == currentBranch && !merged[l.Branch] {
			here = "  ← **you are here**"
		}
		label := l.TitleHint
		if label == "" {
			label = l.Branch
		}
		link := fmt.Sprintf("`%s`", l.Branch)
		if l.PR > 0 {
			if u := prURL(l.PR); u != "" {
				link = fmt.Sprintf("[#%d](%s) `%s`", l.PR, u, l.Branch)
			} else {
				link = fmt.Sprintf("#%d `%s`", l.PR, l.Branch)
			}
		}
		fmt.Fprintf(&b, "* %s %s — %s%s%s\n", check, link, label, suffix, here)
	}
	b.WriteString("\n")
	b.WriteString("> Each PR targets `main`. Diffs are cumulative from trunk through this layer.\n")
	b.WriteString("> Land any PR with your repository's fast-forward mechanism. Merging a higher\n")
	b.WriteString("> layer lands all commits below it; lower PRs close when their tips are on `main`.\n")
	return b.String()
}
