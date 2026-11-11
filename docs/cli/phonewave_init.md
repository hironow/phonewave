## phonewave init

Scan repositories, discover tools, generate routing table

### Synopsis

Scan one or more repositories for tool endpoints, parse SKILL.md manifests, derive a routing table, and generate .phonewave/config.yaml.

```
phonewave init <repo-path> [repo-path...] [flags]
```

### Examples

```
  phonewave init ./sightjack-repo ./paintress-repo ./amadeus-repo
  phonewave init /absolute/path/to/repo
  phonewave init --force ./repo  # overwrite existing config
```

### Options

```
      --force[=true|false]    overwrite existing configuration
  -h, --help[=true|false]     help for init
      --otel-backend string   OTel backend: jaeger, weave
      --otel-entity string    Weave entity/team (required for weave)
      --otel-project string   Weave project (required for weave)
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
