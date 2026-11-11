// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"slices"
	"strings"
	"testing"
)

func TestWaitArgs(t *testing.T) {
	if got, want := WaitArgs(), []string{"wait-for", "dotty-tree"}; !slices.Equal(got, want) {
		t.Errorf("WaitArgs() = %v, want %v", got, want)
	}
}

func TestRegisterArgs(t *testing.T) {
	want := []string{
		"if-shell", "-F",
		"#{==:#{S:#{W:#{P:#{?#{&&:#{@dotty_is_sidebar},#{!=:#{pane_id},%7}},1,}}}},}",
		"set -p -t '%7' @dotty_is_sidebar 1 ; select-pane -t '%7' -T sessions",
		";", "display-message", "-p", "-t", "%7", "#{@dotty_is_sidebar}",
	}
	if got := RegisterArgs("%7"); !slices.Equal(got, want) {
		t.Errorf("RegisterArgs(%%7) =\n%q\nwant\n%q", got, want)
	}
}

func TestFollowArgs(t *testing.T) {
	want := []string{"join-pane", "-d", "-h", "-f", "-l", "32", "-s", "%7", "-t", "@4"}
	if got := FollowArgs("%7", "@4"); !slices.Equal(got, want) {
		t.Errorf("FollowArgs() = %q, want %q", got, want)
	}
}

func TestSwitchArgs(t *testing.T) {
	refocus := []string{";", "if-shell", "-F", "-t", "%7", "#{pane_active}", "last-pane -t '%7'"}
	tests := []struct {
		name                    string
		client, session, window string
		want                    []string
	}{
		{
			name:    "session row",
			client:  "/dev/ttys001",
			session: "$2",
			want:    append([]string{"switch-client", "-c", "/dev/ttys001", "-t", "$2"}, refocus...),
		},
		{
			name:    "window row",
			client:  "/dev/ttys001",
			session: "$2",
			window:  "@9",
			want:    append([]string{"switch-client", "-c", "/dev/ttys001", "-t", "$2:@9"}, refocus...),
		},
		{
			name:    "no client found",
			session: "$2",
			window:  "@9",
			want:    append([]string{"switch-client", "-t", "$2:@9"}, refocus...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SwitchArgs(tt.client, tt.session, tt.window, "%7"); !slices.Equal(got, tt.want) {
				t.Errorf("SwitchArgs() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestToggleArgs(t *testing.T) {
	tests := []struct {
		name string
		st   SidebarState
		want []string
	}{
		{
			name: "hidden shows and focuses",
			st:   SidebarState{Focus: "%1", FocusWindow: "@1"},
			want: []string{
				"set-option", "-g", "@dotty_sidebar", "on",
				";", "split-window", "-h", "-f", "-l", "32", "-t", "@1", "exec dotty tmux sidebar",
			},
		},
		{
			name: "focused hides",
			st:   SidebarState{Focus: "%7", FocusWindow: "@1", Pane: "%7", PaneWindow: "@1"},
			want: []string{"set-option", "-g", "@dotty_sidebar", "off", ";", "kill-pane", "-t", "%7"},
		},
		{
			name: "elsewhere moves here and focuses",
			st:   SidebarState{Focus: "%1", FocusWindow: "@1", Pane: "%7", PaneWindow: "@2"},
			want: []string{"join-pane", "-d", "-h", "-f", "-l", "32", "-s", "%7", "-t", "@1", ";", "select-pane", "-t", "%7"},
		},
		{
			name: "visible unfocused focuses",
			st:   SidebarState{Focus: "%1", FocusWindow: "@1", Pane: "%7", PaneWindow: "@1"},
			want: []string{"select-pane", "-t", "%7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToggleArgs(tt.st); !slices.Equal(got, tt.want) {
				t.Errorf("ToggleArgs() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestEnsureArgs(t *testing.T) {
	got := EnsureArgs("@4")
	if len(got) != 4 || got[0] != "if-shell" || got[1] != "-F" {
		t.Fatalf("EnsureArgs() = %q, want an if-shell -F guard", got)
	}
	if want := "#{&&:#{==:#{@dotty_sidebar},on},#{==:" + sidebarExistsFormat + ",}}"; got[2] != want {
		t.Errorf("guard = %q, want %q", got[2], want)
	}
	if want := "split-window -d -h -f -l 32 -t '@4' 'exec dotty tmux sidebar'"; got[3] != want {
		t.Errorf("command = %q, want %q", got[3], want)
	}
}

// TestSidebarFormatMatchesConf keeps sidebar.conf's follow hooks in step with
// the existence check the Go side uses.
func TestSidebarFormatMatchesConf(t *testing.T) {
	if !strings.Contains(sidebarExistsFormat, "#{m:*dotty tmux sidebar*,#{pane_start_command}}") {
		t.Errorf("sidebarExistsFormat = %q no longer matches the start-command marker", sidebarExistsFormat)
	}
	if !strings.Contains(SidebarCommand, "dotty tmux sidebar") {
		t.Errorf("SidebarCommand = %q no longer matches the start-command glob", SidebarCommand)
	}
}

func TestParseSidebarState(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    SidebarState
		wantErr bool
	}{
		{
			name: "no sidebar",
			out:  "%1" + sep + "@1" + sep + "\n",
			want: SidebarState{Focus: "%1", FocusWindow: "@1"},
		},
		{
			name: "sidebar found",
			out:  "%1" + sep + "@1" + sep + "%7:@2 \n",
			want: SidebarState{Focus: "%1", FocusWindow: "@1", Pane: "%7", PaneWindow: "@2"},
		},
		{
			name: "first of several",
			out:  "%1" + sep + "@1" + sep + "%7:@2 %8:@3 \n",
			want: SidebarState{Focus: "%1", FocusWindow: "@1", Pane: "%7", PaneWindow: "@2"},
		},
		{name: "malformed", out: "nonsense\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSidebarState([]byte(tt.out))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSidebarState() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseSidebarState() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlacement(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "follows a different target",
			out:  strings.Join([]string{"@1", "3", "$1", "@2", "@1:0 @2:1 "}, sep),
			want: FollowArgs("%7", "@2"),
		},
		{
			name: "stays put when in the target with company",
			out:  strings.Join([]string{"@1", "2", "$1", "@1", "@1:0 @2:1 "}, sep),
			want: nil,
		},
		{
			name: "stays put with no target yet",
			out:  strings.Join([]string{"@1", "2", "$1", "", "@1:0 "}, sep),
			want: nil,
		},
		{
			name: "alone retreats to the last-used window",
			out:  strings.Join([]string{"@1", "1", "$1", "@1", "@3:0 @1:0 @2:1 "}, sep),
			want: FollowArgs("%7", "@2"),
		},
		{
			name: "alone retreats to any other window",
			out:  strings.Join([]string{"@1", "1", "$1", "@1", "@1:1 @3:0 "}, sep),
			want: FollowArgs("%7", "@3"),
		},
		{
			name: "alone in the last window closes it",
			out:  strings.Join([]string{"@1", "1", "$1", "@1", "@1:0 "}, sep),
			want: []string{"kill-window", "-t", "@1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePlacement([]byte(tt.out))
			if err != nil {
				t.Fatalf("ParsePlacement() error = %v", err)
			}
			if got := PlaceArgs("%7", p); !slices.Equal(got, tt.want) {
				t.Errorf("PlaceArgs() = %q, want %q", got, tt.want)
			}
		})
	}

	if _, err := ParsePlacement([]byte("x")); err == nil {
		t.Error("ParsePlacement(malformed) error = nil")
	}
	if _, err := ParsePlacement([]byte(strings.Join([]string{"@1", "many", "$1", "", ""}, sep))); err == nil {
		t.Error("ParsePlacement(bad pane count) error = nil")
	}
}

func TestClientViewing(t *testing.T) {
	out := []byte("/dev/ttys001" + sep + "@1\n/dev/ttys002" + sep + "@2\n")
	if got := ClientViewing(out, "@2"); got != "/dev/ttys002" {
		t.Errorf("ClientViewing(@2) = %q, want /dev/ttys002", got)
	}
	if got := ClientViewing(out, "@9"); got != "" {
		t.Errorf("ClientViewing(@9) = %q, want empty", got)
	}
}
