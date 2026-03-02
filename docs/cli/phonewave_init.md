## phonewave init

Scan repositories, discover tools, generate routing table

### Synopsis

Scan one or more repositories for tool endpoints, parse SKILL.md manifests, derive a routing table, and generate phonewave.yaml.

```
phonewave init <repo-path> [repo-path...] [flags]
```

### Examples

```
  phonewave init ./sightjack-repo ./paintress-repo ./amadeus-repo
  phonewave init /absolute/path/to/repo
  phonewave init --force ./repo  # overwrite existing config
```

### Options

```
      --force   overwrite existing configuration
  -h, --help    help for init
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default "phonewave.yaml")
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

