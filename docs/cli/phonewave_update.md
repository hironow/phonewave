## phonewave update

Self-update phonewave to the latest release

### Synopsis

Self-update phonewave to the latest GitHub release.

Downloads the latest release, verifies the checksum, and replaces
the current binary. Use --check to only check for updates without
installing.

```
phonewave update [flags]
```

### Examples

```
  # Check for updates
  phonewave update --check

  # Update to the latest version
  phonewave update
```

### Options

```
  -C, --check   Check for updates without installing
  -h, --help    help for update
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

