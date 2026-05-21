package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hironow/phonewave/internal/session"
)

// newMCPCommand exposes `phonewave mcp` as a stdio MCP server entry
// point for the refs/issues/0027 jun15 MCP pivot Phase 2d. A claude
// code interactive session loads this binary via --mcp-config to
// query phonewave's runtime data plane (outbox / inbox / dead-letter
// queue depth) as visibility tools alongside the other four tools'
// MCP servers.
//
// Phase 2d MVP exposes phonewave.ping + 2 stubs (outbox_status,
// inbox_status). Real tool wiring against the courier daemon state
// lands in subsequent commits on feat/jun15-mcp-pivot.
//
// Note: phonewave does NOT invoke `claude -p` in any production
// path, so this MCP server is purely additive (= visibility tools)
// and there is no deprecated CLI subcommand to redirect operators
// away from.
func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run phonewave as an MCP server over stdio (refs/issues/0027 Phase 2d MVP)",
		Long: `Start a Model Context Protocol server reading JSON-RPC 2.0
messages on stdin and writing responses on stdout.

Designed for embedding in a claude code interactive session via
--mcp-config to query phonewave's courier daemon state without
shelling out to 'phonewave status' / 'phonewave metrics'. The
session can attach this MCP server alongside the other four tools'
MCP servers (paintress / sightjack / amadeus / dominator).

Phase 2d MVP scope: phonewave.ping + 2 stubs (phonewave.outbox_status,
phonewave.inbox_status). Real wiring against the courier daemon
state lookup ships in subsequent commits on the feat/jun15-mcp-pivot
branch.

phonewave never invoked 'claude -p' in any production path, so this
MCP server is purely additive (= visibility) and complements the
other four tools' MCP servers which replace deprecated LLM
invocation paths.`,
		Example: `  # Launch claude code with all five MCP servers attached
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
			srv := session.NewMCPServer(cmd.InOrStdin(), cmd.OutOrStdout(), nil)
			return srv.Serve(cmd.Context())
		},
	}
}
