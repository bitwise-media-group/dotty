// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// line joins one TreeFormat row.
func line(fields ...string) string {
	return strings.Join(fields, sep) + "\n"
}

func TestParseTree(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []Session
	}{
		{
			name: "empty",
			out:  "",
			want: nil,
		},
		{
			name: "windows group under their session with glyphs and spaces intact",
			out: line("$1", "dotty", "/r/dotty", "@1", "1", "  nvim", "1", "", "0") +
				line("$1", "dotty", "/r/dotty", "@2", "2", "\U000F4036  claude", "0", "attention", "0"),
			want: []Session{{ID: "$1", Name: "dotty", Path: "/r/dotty", Windows: []Window{
				{ID: "@1", Index: "1", Name: "  nvim", Active: true},
				{ID: "@2", Index: "2", Name: "\U000F4036  claude", Status: StatusAttention},
			}}},
		},
		{
			name: "unknown status reads as none",
			out:  line("$1", "a", "/a", "@1", "1", "w", "1", "bogus", "0"),
			want: []Session{{ID: "$1", Name: "a", Path: "/a", Windows: []Window{
				{ID: "@1", Index: "1", Name: "w", Active: true},
			}}},
		},
		{
			name: "malformed lines are skipped",
			out: "garbage\n" +
				line("$1", "a", "/a", "@1", "1", "w", "1", "", "0") +
				line("", "nameless", "/n", "@9", "1", "w", "1", "", "0") +
				line("$2", "b", "/b", "@2", "1", "w") +
				line("$3", "c", "/c", "", "1", "w", "1", "", "0"),
			want: []Session{{ID: "$1", Name: "a", Path: "/a", Windows: []Window{
				{ID: "@1", Index: "1", Name: "w", Active: true},
			}}},
		},
		{
			name: "sorted by name with worktree sessions after their repository",
			out: line("$3", "zeta", "/z", "@3", "1", "w", "1", "", "0") +
				line("$2", "dotty-feat-x", "/w/x", "@2", "1", "w", "1", "", "0") +
				line("$1", "dotty", "/r/dotty", "@1", "1", "w", "1", "", "0"),
			want: []Session{
				{ID: "$1", Name: "dotty", Path: "/r/dotty", Windows: []Window{{ID: "@1", Index: "1", Name: "w", Active: true}}},
				{ID: "$2", Name: "dotty-feat-x", Path: "/w/x", Windows: []Window{{ID: "@2", Index: "1", Name: "w", Active: true}}},
				{ID: "$3", Name: "zeta", Path: "/z", Windows: []Window{{ID: "@3", Index: "1", Name: "w", Active: true}}},
			},
		},
		{
			name: "current session first",
			out: line("$1", "alpha", "/a", "@1", "1", "w", "1", "", "0") +
				line("$2", "beta", "/b", "@2", "1", "w", "1", "", "0") +
				line("$2", "beta", "/b", "@3", "2", "v", "0", "", "1"),
			want: []Session{
				{ID: "$2", Name: "beta", Path: "/b", Windows: []Window{
					{ID: "@2", Index: "1", Name: "w", Active: true},
					{ID: "@3", Index: "2", Name: "v", Current: true},
				}},
				{ID: "$1", Name: "alpha", Path: "/a", Windows: []Window{{ID: "@1", Index: "1", Name: "w", Active: true}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseTree([]byte(tt.out)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTree() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestSessionStatus(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []string
		want      string
		wantCount int
	}{
		{name: "none", statuses: []string{"", ""}, want: "", wantCount: 0},
		{name: "waiting counted", statuses: []string{StatusWaiting, "", StatusWaiting}, want: StatusWaiting, wantCount: 2},
		{
			name:      "attention outranks waiting",
			statuses:  []string{StatusWaiting, StatusAttention, StatusWaiting},
			want:      StatusAttention,
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Session
			for _, st := range tt.statuses {
				s.Windows = append(s.Windows, Window{Status: st})
			}
			if got, n := s.Status(); got != tt.want || n != tt.wantCount {
				t.Errorf("Status() = %q, %d; want %q, %d", got, n, tt.want, tt.wantCount)
			}
		})
	}
}

// outRunner returns fixed output for every call, recording each.
type outRunner struct {
	out   string
	err   error
	calls [][]string
}

func (o *outRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	o.calls = append(o.calls, append([]string{name}, args...))
	return []byte(o.out), o.err
}

func TestListTree(t *testing.T) {
	r := &outRunner{out: line("$1", "a", "/a", "@1", "1", "w", "1", "waiting", "0")}
	got, err := ListTree(context.Background(), r)
	if err != nil {
		t.Fatalf("ListTree() error = %v", err)
	}
	if len(got) != 1 || got[0].Windows[0].Status != StatusWaiting {
		t.Errorf("ListTree() = %+v", got)
	}
	want := []string{"tmux", "list-windows", "-a", "-F", TreeFormat}
	if len(r.calls) != 1 || !slices.Equal(r.calls[0], want) {
		t.Errorf("calls = %v, want [%v]", r.calls, want)
	}

	r = &outRunner{err: errors.New("no server running")}
	if _, err := ListTree(context.Background(), r); err == nil {
		t.Error("ListTree() error = nil, want the runner failure")
	}
}
