## phonewave mcp

Run phonewave as an MCP server over stdio (refs/issues/0027 Phase 2d MVP)

### Synopsis

Start a Model Context Protocol server reading JSON-RPC 2.0
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
invocation paths.

```
phonewave mcp [flags]
```

### Examples

```
  # Launch claude code with all five MCP servers attached
  claude --mcp-config '{
    "paintress":{"command":"paintress","args":["mcp"]},
    "sightjack":{"command":"sightjack","args":["mcp"]},
    "amadeus":{"command":"amadeus","args":["mcp"]},
    "dominator":{"command":"dominator","args":["mcp"]},
    "phonewave":{"command":"phonewave","args":["mcp"]}
  }'

  # Pipe a tools/list request manually (for debugging)
  echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | phonewave mcp
```

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default ".phonewave/config.yaml")
      --linear          Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

