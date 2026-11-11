## phonewave doctor

Verify ecosystem health

### Synopsis

Check ecosystem health: verify paths, endpoints, SKILL.md spec compliance, PID conflicts, and daemon status.

With --all, runs all 4 tool doctors (sightjack, paintress, amadeus) against
the specified repo path and presents a unified report with cross-tool checks.

```
phonewave doctor [repo-path] [flags]
```

### Examples

```
  # Run phonewave-only health check
  phonewave doctor

  # Run unified health check across all 4 tools
  phonewave doctor --all /path/to/repo

  # JSON output for scripting
  phonewave doctor -o json

  # Auto-fix repairable issues
  phonewave doctor --repair
```

### Options

```
      --all[=true|false]      Run unified doctor across all 4 TAP tools
  -h, --help[=true|false]     help for doctor
      --repair[=true|false]   Auto-fix repairable issues
```

### Options inherited from parent commands

```
  -c, --config string           Path to phonewave config file (default ".phonewave/config.yaml")
      --linear[=true|false]     Use Linear MCP for issue tracking (default: wave-centric mode)
      --no-color[=true|false]   Disable colored output (respects NO_COLOR env)
  -o, --output string           Output format: text, json (default "text")
  -q, --quiet[=true|false]      Suppress all stderr output
  -v, --verbose[=true|false]    Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)  - D-Mail courier daemon
