## phonewave add

Add a new repository to the ecosystem

### Synopsis

Add a new repository to the phonewave ecosystem, scan its endpoints, and update the routing table.

```
phonewave add <repo-path> [flags]
```

### Examples

```
  phonewave add ./new-repo
  phonewave add /absolute/path/to/repo
```

### Options

```
  -h, --help   help for add
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
