# Quickstart

## Install

From source:

```bash
go build -o go-doctor ./cmd/go-doctor
```

With `go install`:

```bash
go install github.com/sstehniy/go-doctor/cmd/go-doctor@latest
```

Pinned release:

```bash
go install github.com/sstehniy/go-doctor/cmd/go-doctor@v1.2.3
```

From a release archive:

1. Download the matching `go-doctor_<version>_<os>_<arch>.(tar.gz|zip)` artifact.
2. Extract it.
3. Move `go-doctor` (or `go-doctor.exe`) into your `PATH`.

## First scan

```bash
go-doctor .
```

See all flags and examples:

```bash
go-doctor --help
```

Generated CLI reference: [cli/go-doctor.md](./cli/go-doctor.md)

## Output formats

```bash
go-doctor --format text .
go-doctor --format json .
go-doctor --format sarif --output results.sarif .
```

## Shell completion

```bash
go-doctor completion bash
go-doctor completion zsh
```

## Diff mode

```bash
go-doctor --diff .
go-doctor --diff origin/main .
go-doctor --diff --diff-govulncheck changed-modules-only .
```

## Rule discovery

```bash
go-doctor --list-rules
go-doctor --enable mod/not-tidy,build/mod-readonly-failure .
go-doctor --disable fmt/not-gofmt .
```

## Timeout and retries

- `--timeout` sets the global scan deadline.
- Each analyzer run is executed with the same timeout budget and hard-cancelled when exceeded.
- Retry policy is deterministic and conservative:
  - one retry (`2` total attempts)
  - only for built-in read-only analyzers (`repo-hygiene`, `govet`, `staticcheck`, `govulncheck`, `golangci-lint`, `custom`)
  - retries trigger only on timeout

## Baseline adoption

```bash
go-doctor --baseline .go-doctor-baseline.json --fail-on warning .
```

For full onboarding steps, see [adoption.md](./adoption.md).
