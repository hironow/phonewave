package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// newMCPCommand exposes `phonewave mcp` as a stdio MCP server entry
// point for the refs/issues/0027 jun15 MCP pivot Phase 2d. A Claude
// Code interactive session loads this binary via --mcp-config to
// query phonewave's runtime data plane (outbox / inbox / dead-letter
// queue depth) as visibility tools alongside the other four tools'
// MCP servers.
//
// Exposes phonewave.ping plus the courier data-plane visibility tools
// outbox_status / inbox_status (real reads of the SQLite delivery
// store + outbox/inbox queue depths across configured repositories).
//
// Note: phonewave does NOT invoke `claude -p` in any production
// path, so this MCP server is purely additive (= visibility tools)
// and there is no deprecated CLI subcommand to redirect operators
// away from.
func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run phonewave as an MCP server over stdio (courier data-plane visibility tools)",
		Long: `Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a Claude Code interactive session via
--mcp-config to query phonewave's courier daemon state without
shelling out to 'phonewave status' / 'phonewave metrics'. The
session can attach this MCP server alongside the other four tools'
MCP servers (paintress / sightjack / amadeus / dominator).

Exposes phonewave.ping plus phonewave.outbox_status and
phonewave.inbox_status, which read the courier daemon's runtime state
(outbox/inbox queue depth + dead-letter count from the SQLite
delivery store) across all configured repositories.

phonewave never invoked 'claude -p' in any production path, so this
MCP server is purely additive (= visibility) and complements the
other four tools' MCP servers which replace deprecated LLM
invocation paths.`,
		Example: `  # Launch Claude Code with all five MCP servers attached
  claude --mcp-config '{
    "paintress":{"command":"paintress","args":["mcp"]},
    "sightjack":{"command":"sightjack","args":["mcp"]},
    "amadeus":{"command":"amadeus","args":["mcp"]},
    "dominator":{"command":"dominator","args":["mcp"]},
    "phonewave":{"command":"phonewave","args":["mcp"]}
  }'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | phonewave mcp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			configPath := filepath.Join(cwd, domain.StateDir, domain.ConfigFile)
			srv := session.NewMCPServer(cmd.InOrStdin(), cmd.OutOrStdout(), nil).WithConfigPath(configPath)
			return srv.Serve(cmd.Context())
		},
	}
}
