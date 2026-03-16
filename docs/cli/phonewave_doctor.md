## phonewave doctor

Verify ecosystem health

### Synopsis

Check ecosystem health: verify paths, endpoints, SKILL.md spec compliance, PID conflicts, and daemon status.

```
phonewave doctor [flags]
```

### Examples

```
  # Run ecosystem health check
  phonewave doctor

  # JSON output for scripting
  phonewave doctor -o json

  # Auto-fix repairable issues
  phonewave doctor --repair
```

### Options

```
  -h, --help     help for doctor
      --repair   Auto-fix repairable issues
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default ".phonewave/config.yaml")
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

