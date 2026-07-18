// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strings"
)

// Commit is a single commit for picklists / PR metadata.
type Commit struct {
	SHA     string
	Subject string
	Body    string
}

// LayerCommits returns commits unique to this layer: parentTip..branch
// (or trunk..branch for the bottom layer).
func LayerCommits(ctx context.Context, r Runner, parentRev, branch string) ([]Commit, error) {
	spec := parentRev + ".." + branch
	out, err := r.Output(ctx, "git", "log", "--reverse", "--format=%H%x00%s%x00%b%x1e", spec)
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", spec, err)
	}
	raw := strings.TrimSuffix(string(out), "\x1e")
	var commits []Commit
	for rec := range strings.SplitSeq(raw, "\x1e") {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x00", 3)
		c := Commit{}
		if len(parts) > 0 {
			c.SHA = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			c.Subject = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			c.Body = strings.TrimSpace(parts[2])
		}
		if c.SHA != "" {
			commits = append(commits, c)
		}
	}
	return commits, nil
}

// ParentRevForLayer returns the revision that is the exclusive lower bound of
// the layer's exclusive commits.
func ParentRevForLayer(s Stack, i int, trunk Trunk) string {
	if i <= 0 {
		return trunk.Ref()
	}
	return s.Layers[i-1].Branch
}
