## phonewave update

Update phonewave to the latest version

### Synopsis

Check for and install the latest version of phonewave from GitHub releases.

```
phonewave update [flags]
```

### Examples

```
  # Check for updates without installing
  phonewave update -C

  # Update to latest version
  phonewave update
```

### Options

```
  -C, --check   Check for updates without installing
  -h, --help    help for update
```

### Options inherited from parent commands

```
  -c, --config string   Path to phonewave config file (default "phonewave.yaml")
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Log all delivery events to stderr
```

### SEE ALSO

* [phonewave](phonewave.md)  - D-Mail courier daemon
