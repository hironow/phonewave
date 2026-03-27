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
      --all      Run unified doctor across all 4 TAP tools
  -h, --help     help for doctor
      --repair   Auto-fix repairable issues
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

