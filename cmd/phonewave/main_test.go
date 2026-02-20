package main

import (
	"testing"
)

func TestExtractSubcommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCmd  string
		wantRest []string
	}{
		{
			name:     "no args",
			args:     nil,
			wantCmd:  "",
			wantRest: nil,
		},
		{
			name:     "version flag",
			args:     []string{"--version"},
			wantCmd:  "version",
			wantRest: nil,
		},
		{
			name:     "help flag",
			args:     []string{"--help"},
			wantCmd:  "help",
			wantRest: nil,
		},
		{
			name:     "init with paths",
			args:     []string{"init", "./repo-a", "./repo-b"},
			wantCmd:  "init",
			wantRest: []string{"./repo-a", "./repo-b"},
		},
		{
			name:     "add with path",
			args:     []string{"add", "./repo-c"},
			wantCmd:  "add",
			wantRest: []string{"./repo-c"},
		},
		{
			name:     "remove with path",
			args:     []string{"remove", "./repo-c"},
			wantCmd:  "remove",
			wantRest: []string{"./repo-c"},
		},
		{
			name:     "sync no args",
			args:     []string{"sync"},
			wantCmd:  "sync",
			wantRest: nil,
		},
		{
			name:     "doctor no args",
			args:     []string{"doctor"},
			wantCmd:  "doctor",
			wantRest: nil,
		},
		{
			name:     "run with flags",
			args:     []string{"run", "--verbose"},
			wantCmd:  "run",
			wantRest: []string{"--verbose"},
		},
		{
			name:     "status no args",
			args:     []string{"status"},
			wantCmd:  "status",
			wantRest: nil,
		},
		{
			name:     "unknown args returned as rest",
			args:     []string{"./some-path"},
			wantCmd:  "",
			wantRest: []string{"./some-path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotRest := extractSubcommand(tt.args)
			if gotCmd != tt.wantCmd {
				t.Errorf("subcmd = %q, want %q", gotCmd, tt.wantCmd)
			}
			if len(gotRest) != len(tt.wantRest) {
				t.Errorf("rest = %v, want %v", gotRest, tt.wantRest)
				return
			}
			for i := range gotRest {
				if gotRest[i] != tt.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q", i, gotRest[i], tt.wantRest[i])
				}
			}
		})
	}
}

func TestExtractPaths(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "paths only",
			args: []string{"./repo-a", "./repo-b"},
			want: []string{"./repo-a", "./repo-b"},
		},
		{
			name: "mixed flags and paths",
			args: []string{"--verbose", "./repo-a", "--dry-run"},
			want: []string{"./repo-a"},
		},
		{
			name: "stop at double dash",
			args: []string{"./repo-a", "--", "./repo-b"},
			want: []string{"./repo-a"},
		},
		{
			name: "no paths",
			args: []string{"--verbose"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPaths(tt.args)
			if len(got) != len(tt.want) {
				t.Errorf("extractPaths = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractPaths[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
