## go-doctor

Check Go repositories for code health issues.

### Synopsis

go-doctor scans Go repositories, normalizes findings across analyzers, and reports a clear health signal for local work and CI.

```
go-doctor [flags] [target]
```

### Examples

```
go-doctor .
go-doctor --format json .
go-doctor --format sarif --output results.sarif .
go-doctor --diff .
go-doctor --diff origin/main .
go-doctor --list-rules
go-doctor --baseline .go-doctor-baseline.json --fail-on warning .
go-doctor completion zsh
```

### Options

```
      --config string             Path to config file. Defaults to auto-discovery in the target.
      --format string             Output format: text, json, or sarif. Defaults to config or text.
      --output string             Write rendered output to this path in addition to stdout.
      --verbose                   Show verbose output in text mode.
      --quiet                     Reduce summary noise in text mode.
      --no-score                  Disable score output, overriding config.
      --fail-on string            Fail threshold: none, info, warning, or error.
      --diff string               Diff base ('auto' or an explicit ref). Bare --diff uses auto.
      --diff-govulncheck string   Diff govulncheck mode: skip or changed-modules-only.
      --packages csv              Comma-separated package patterns to scan.
      --modules csv               Comma-separated module roots to scan.
      --timeout duration          Global scan timeout, such as 30s or 2m. (default 0s)
      --concurrency int           Max analyzer concurrency. Defaults to config or CPU-based default.
      --enable csv                Comma-separated rules or selectors to enable.
      --disable csv               Comma-separated rules or selectors to disable.
      --baseline string           Baseline file path for adoption workflows.
      --no-baseline               Ignore any configured baseline for this run.
      --list-rules                List available rules and selectors, then exit.
      --version                   Print version and exit.
  -h, --help                      help for go-doctor
```

### SEE ALSO

* [go-doctor completion](go-doctor_completion.md)	 - Print shell completion scripts.

