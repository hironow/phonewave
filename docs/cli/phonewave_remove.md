## phonewave remove

Remove a repository from the ecosystem

### Synopsis

Remove a repository from the phonewave ecosystem and update the routing table.

```
phonewave remove <repo-path> [flags]
```

### Examples

```
  phonewave remove ./old-repo
  phonewave remove /absolute/path/to/repo
```

### Options

```
  -h, --help   help for remove
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
