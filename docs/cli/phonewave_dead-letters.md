## phonewave dead-letters

Manage dead-lettered delivery items

### Synopsis

Inspect and manage delivery items that have exceeded the maximum retry count.

```
phonewave dead-letters [flags]
```

### Options

```
  -h, --help[=true|false]   help for dead-letters
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
* [phonewave dead-letters purge](phonewave_dead-letters_purge.md)  - Purge dead-lettered delivery items
