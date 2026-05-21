## phonewave dead-letters purge

Purge dead-lettered delivery items

### Synopsis

Remove delivery items that have permanently failed (exceeded max retry count).

By default, runs in dry-run mode showing how many items would be purged.
Pass --execute to actually delete dead-lettered items.

```
phonewave dead-letters purge [flags]
```

### Examples

```
  # Dry-run: show dead-letter count
  phonewave dead-letters purge

  # Delete dead-lettered items (with confirmation)
  phonewave dead-letters purge --execute

  # Delete without confirmation
  phonewave dead-letters purge --execute --yes
```

### Options

```
      --execute   Execute purge (default: dry-run)
  -h, --help      help for purge
      --yes       Skip confirmation prompt
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

* [phonewave dead-letters](phonewave_dead-letters.md)	 - Manage dead-lettered delivery items

