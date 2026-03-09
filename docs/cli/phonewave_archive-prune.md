## phonewave archive-prune

Prune expired event files

### Synopsis

Prune expired event files from the events directory.

Lists event files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.

```
phonewave archive-prune [path] [flags]
```

### Examples

```
  # Dry-run: list expired files (default 30 days)
  phonewave archive-prune

  # Delete expired files
  phonewave archive-prune --execute

  # Specific project directory
  phonewave archive-prune /path/to/project --execute

  # JSON output for scripting
  phonewave archive-prune -o json

  # Custom retention period
  phonewave archive-prune --days 7 --execute
```

### Options

```
  -d, --days int   Retention days (default 30)
  -n, --dry-run    Dry-run mode (default behavior, explicit for scripting)
  -x, --execute    Execute pruning (default: dry-run)
  -h, --help       help for archive-prune
  -y, --yes        Skip confirmation prompt
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default "phonewave.yaml")
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

