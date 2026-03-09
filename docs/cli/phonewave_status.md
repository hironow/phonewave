## phonewave status

Show daemon and delivery status

### Synopsis

Show daemon state, uptime, watched directories, route count, error queue, and 24h delivery statistics.

```
phonewave status [path] [flags]
```

### Examples

```
  phonewave status
  phonewave status /path/to/project
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default "phonewave.yaml")
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)  - D-Mail courier daemon
