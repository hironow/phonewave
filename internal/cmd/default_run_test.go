package cmd
// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"testing"
)

func TestNeedsDefaultRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		// No args → default to run
		{"empty args", nil, true},
		{"empty slice", []string{}, true},

		// Explicit subcommands → no default
		{"explicit run", []string{"run"}, false},
		{"explicit init", []string{"init"}, false},
		{"explicit add", []string{"add"}, false},
		{"explicit remove", []string{"remove"}, false},
		{"explicit sync", []string{"sync"}, false},
		{"explicit doctor", []string{"doctor"}, false},
		{"explicit status", []string{"status"}, false},
		{"explicit clean", []string{"clean"}, false},
		{"explicit archive-prune", []string{"archive-prune"}, false},
		{"explicit version", []string{"version"}, false},
		{"explicit update", []string{"update"}, false},
		{"explicit help", []string{"help"}, false},
		{"explicit completion", []string{"completion"}, false},

		// Root flags that suppress default
		{"--version", []string{"--version"}, false},
		{"--help", []string{"--help"}, false},
		{"-h", []string{"-h"}, false},

		// Persistent flags before subcommand → still finds subcommand
		{"verbose then run", []string{"-v", "run"}, false},
		{"config then run", []string{"-c", "cfg.yaml", "run"}, false},
		{"config=val then run", []string{"-c=cfg.yaml", "run"}, false},
		{"output then run", []string{"-o", "json", "run"}, false},

		// Persistent flags only → default to run
		{"verbose only", []string{"-v"}, true},
		{"config only", []string{"-c", "cfg.yaml"}, true},
		{"long verbose only", []string{"--verbose"}, true},
		{"long config only", []string{"--config", "cfg.yaml"}, true},
		{"output only", []string{"--output", "json"}, true},

		// Unknown flags → default to run (they'll be run's flags)
		{"unknown flag", []string{"--some-flag"}, true},
		{"unknown flag with value", []string{"--some-flag", "value"}, true},

		// -- terminator
		{"double dash only", []string{"--"}, true},
		{"double dash then subcommand", []string{"--", "run"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewRootCommand()
			got := NeedsDefaultRun(rootCmd, tt.args)
			if got != tt.want {
				t.Errorf("NeedsDefaultRun(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
