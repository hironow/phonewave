## phonewave add

Add a new repository to the ecosystem

### Synopsis

Add a new repository to the phonewave ecosystem, scan its endpoints, and update the routing table.

```
phonewave add <repo-path> [flags]
```

### Examples

```
  phonewave add ./new-repo
  phonewave add /absolute/path/to/repo
```

### Options

```
  -h, --help[=true|false]   help for add
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
