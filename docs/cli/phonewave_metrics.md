## phonewave metrics

Output delivery health time-series as JSON

### Synopsis

Read the JSONL event store and output a bucketed delivery health
time-series to stdout. Suitable for dashboard consumption, piping to jq,
or rendering with terminal tools (sampler, wtf).

```
phonewave metrics [flags]
```

### Examples

```
  # Default: last 7 days at hourly buckets
  phonewave metrics

  # Last 24 hours at 15-minute buckets
  phonewave metrics --window 24h --bucket 15m

  # Pipe to jq for totals only
  phonewave metrics | jq '.totals'
```

### Options

```
      --bucket string   Bucket size (e.g. 15m, 1h) (default "1h")
  -h, --help            help for metrics
      --window string   Time window (e.g. 24h, 168h) (default "168h")
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

