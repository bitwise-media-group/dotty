// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func prViewStateCmd(pr string) string {
	return "gh pr view " + pr + " --repo acme/widgets --json state --jq .state"
}

func TestLandedLayers(t *testing.T) {
	cases := []struct {
		name   string
		rows   []LayerStatus
		states map[string]string // PR number → gh state
		errs   map[string]error
		want   []string
		absent string // command that must not be issued
	}{
		{
			name: "a tip behind trunk landed without asking the remote",
			rows: []LayerStatus{{Layer: Layer{Branch: "feat-a", PR: 77}, Relation: RelMerged}},
			want: []string{"feat-a"},
			// Local history already proves it; no gh round trip.
			absent: prViewStateCmd("77"),
		},
		{
			name:   "a tip identical to trunk landed when its PR is merged",
			rows:   []LayerStatus{{Layer: Layer{Branch: "feat-a", PR: 77}, Relation: RelIdentical}},
			states: map[string]string{"77": "MERGED\n"},
			want:   []string{"feat-a"},
		},
		{
			name:   "an identical tip with an open PR stays",
			rows:   []LayerStatus{{Layer: Layer{Branch: "feat-a", PR: 77}, Relation: RelIdentical}},
			states: map[string]string{"77": "OPEN\n"},
			want:   nil,
		},
		{
			name:   "an identical tip with no PR is an empty layer, not landed work",
			rows:   []LayerStatus{{Layer: Layer{Branch: "feat-a"}, Relation: RelIdentical}},
			want:   nil,
			absent: prViewStateCmd("0"),
		},
		{
			name: "an unreachable remote leaves the layer alone",
			rows: []LayerStatus{{Layer: Layer{Branch: "feat-a", PR: 77}, Relation: RelIdentical}},
			errs: map[string]error{prViewStateCmd("77"): errors.New("dial tcp: no route to host")},
			want: nil,
		},
		{
			name: "open layers above a landed bottom are untouched",
			rows: []LayerStatus{
				{Layer: Layer{Branch: "feat-a", PR: 77}, Relation: RelIdentical},
				{Layer: Layer{Branch: "feat-b", PR: 78}, Relation: RelFF},
				{Layer: Layer{Branch: "feat-c"}, Relation: RelDiverged},
			},
			states: map[string]string{"77": "MERGED\n"},
			want:   []string{"feat-a"},
			absent: prViewStateCmd("78"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outputs := map[string]string{fakeRemoteURLCmd: fakeRemoteURL}
			for pr, state := range c.states {
				outputs[prViewStateCmd(pr)] = state
			}
			fake := &ghFakeRunner{outputs: outputs, errs: c.errs}
			got := LandedLayers(context.Background(), fake, c.rows, "upstream")
			if !slices.Equal(got, c.want) {
				t.Errorf("LandedLayers() = %q, want %q", got, c.want)
			}
			if c.absent != "" && slices.Contains(fake.calls, c.absent) {
				t.Errorf("calls = %q, want %q absent", fake.calls, c.absent)
			}
		})
	}
}
