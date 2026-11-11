## phonewave

D-Mail courier daemon

### Synopsis

Phonewave routes D-Mails between AI agent tool repositories via file-based message passing.

```
phonewave [flags]
```

### Options

```
  -c, --config string           Path to phonewave config file (default ".phonewave/config.yaml")
  -h, --help[=true|false]       help for phonewave
      --linear[=true|false]     Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color[=true|false]   Disable colored output (respects NO_COLOR env)
  -o, --output string           Output format: text, json (default "text")
  -q, --quiet[=true|false]      Suppress all stderr output
  -v, --verbose[=true|false]    Log all delivery events to stderr
```

### SEE ALSO

* [phonewave add](phonewave_add.md)  - Add a new repository to the ecosystem
* [phonewave archive-prune](phonewave_archive-prune.md)  - Prune expired event files
* [phonewave clean](phonewave_clean.md)  - Remove runtime state from .phonewave/
* [phonewave dead-letters](phonewave_dead-letters.md)  - Manage dead-lettered delivery items
* [phonewave doctor](phonewave_doctor.md)  - Verify ecosystem health
* [phonewave init](phonewave_init.md)  - Scan repositories, discover tools, generate routing table
* [phonewave mcp](phonewave_mcp.md)  - Run phonewave as an MCP server over stdio (courier data-plane visibility tools)
* [phonewave metrics](phonewave_metrics.md)  - Output delivery health time-series as JSON
* [phonewave remove](phonewave_remove.md)  - Remove a repository from the ecosystem
* [phonewave run](phonewave_run.md)  - Start the courier daemon
* [phonewave status](phonewave_status.md)  - Show daemon and delivery status
* [phonewave sync](phonewave_sync.md)  - Re-scan all repositories, reconcile routing table
* [phonewave update](phonewave_update.md)  - Self-update phonewave to the latest release
* [phonewave version](phonewave_version.md)  - Print version, commit, and build information
