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
  phonewave run --verbose

  # Dry run (detect events, don't deliver)
  phonewave run --dry-run

  # With retry interval
  phonewave run --retry-interval 120s

  # With tracing enabled
  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run --verbose
```

### Options

```
      --dry-run                   Detect events without delivering
  -h, --help                      help for run
      --max-retries int           Maximum retry attempts per failed D-Mail (default 10)
      --retry-interval duration   Error queue retry interval (0 to disable) (default 1m0s)
```

### Options inherited from parent commands

```
  -v, --verbose   Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

