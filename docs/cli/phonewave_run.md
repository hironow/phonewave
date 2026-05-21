## phonewave run

Start the courier daemon

### Synopsis

Start the phonewave courier daemon. Watches outbox directories for new D-Mails and delivers them to the correct inbox(es) based on the routing table.

```
phonewave run [flags]
```

### Examples

```
  # Start daemon (foreground, verbose)
  phonewave run -v

  # Dry run (detect events, don't deliver)
  phonewave run -n

  # With retry interval
  phonewave run -r 120s

  # With tracing enabled
  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run -v
```

### Options

```
  -n, --dry-run                   Detect events without delivering
  -h, --help                      help for run
      --idle-timeout duration     idle timeout — exit after no activity (0 = 24h safety cap, negative = disable) (default 30m0s)
  -m, --max-retries int           Maximum retry attempts per failed D-Mail (default 10)
  -r, --retry-interval duration   Error queue retry interval (0 to disable) (default 1m0s)
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

