// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestPRText(t *testing.T) {
	const viewCmd = "gh pr view 7 --repo acme/widgets --json title,body"
	cases := []struct {
		name    string
		view    string
		viewErr error
		want    PRContent
		wantErr bool
	}{
		{
			name: "reads the current title and body",
			view: `{"title":"feat: a","body":"the description"}`,
			want: PRContent{Title: "feat: a", Body: "the description"},
		},
		{
			name: "an empty body is not an error",
			view: `{"title":"feat: a","body":""}`,
			want: PRContent{Title: "feat: a"},
		},
		{
			name:    "unparseable json is an error",
			view:    "not json",
			wantErr: true,
		},
		{
			name:    "surfaces the gh failure",
			viewErr: errors.New("gh: not found"),
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd: fakeRemoteURL,
				viewCmd:          c.view,
			}}
			if c.viewErr != nil {
				fake.errs = map[string]error{viewCmd: c.viewErr}
			}
			got, err := PRText(context.Background(), fake, 7, "upstream")
			if (err != nil) != c.wantErr {
				t.Fatalf("PRText() error = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("PRText() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestUpdatePR(t *testing.T) {
	const editCmd = "gh pr edit 7 --repo acme/widgets --title feat: a --body the description"
	fake := &ghFakeRunner{outputs: map[string]string{fakeRemoteURLCmd: fakeRemoteURL}}
	want := PRContent{Title: "feat: a", Body: "the description"}
	if err := UpdatePR(context.Background(), fake, 7, want, "upstream"); err != nil {
		t.Fatalf("UpdatePR() error: %v", err)
	}
	// The title travels with the body: a stacked layer owns both.
	if !slices.Contains(fake.calls, editCmd) {
		t.Errorf("calls = %q, want %q", fake.calls, editCmd)
	}
}
