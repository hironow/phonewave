## phonewave clean

Remove runtime state from .phonewave/

### Synopsis

Remove runtime state files from the .phonewave/ directory.

Removes: delivery.log, .run/, watch.pid, watch.started, events/
Also removes: skills-ref Python venv from temp directory (if present)
Preserves: .phonewave/config.yaml and .phonewave/.gitignore

```
phonewave clean [path] [flags]
```

### Examples

```
  # Clean current directory
  phonewave clean

  # Clean a specific project
  phonewave clean /path/to/project

  # Skip confirmation prompt
  phonewave clean --yes
```

### Options

```
  -h, --help   help for clean
      --yes    skip confirmation prompt
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
