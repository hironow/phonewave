## phonewave clean

Remove runtime state from .phonewave/

### Synopsis

Remove runtime state files from the .phonewave/ directory.

Removes: delivery.log, errors/, .run/, watch.pid, watch.started, events/
Preserves: phonewave.yaml (config) and .phonewave/.gitignore

```
phonewave clean [flags]
```

### Examples

```
  phonewave clean
  phonewave clean --yes
```

### Options

```
  -h, --help   help for clean
      --yes    skip confirmation prompt
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default "phonewave.yaml")
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)  - D-Mail courier daemon
