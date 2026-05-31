## phonewave status

Show daemon and delivery status

### Synopsis

Show daemon state, uptime, watched directories, route count,
error queue, and 24h delivery statistics.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output to stdout.

```
phonewave status [path] [flags]
```

### Examples

```
  # Show status for default config location
  phonewave status

  # Show status for a specific project
  phonewave status /path/to/project

  # JSON output for scripting
  phonewave status -o json
```

### Options

```
  -h, --help   help for status
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

* [phonewave](phonewave.md)  - D-Mail courier daemon
