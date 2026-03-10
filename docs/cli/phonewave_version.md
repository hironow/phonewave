## phonewave version

Print version, commit, and build information

### Synopsis

Print version, commit hash, build date, Go version, and OS/arch.

By default outputs a human-readable single line. Use --json
for structured output suitable for scripts and CI.

```
phonewave version [flags]
```

### Examples

```
  phonewave version
  phonewave version -j
```

### Options

```
  -h, --help   help for version
  -j, --json   Output version info as JSON
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default ".phonewave/config.yaml")
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)	 - D-Mail courier daemon

