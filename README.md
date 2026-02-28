# Go Doctor

`go-doctor` is a Go code health checker for Go repositories.

Goal: one command that scans a repo, surfaces high-signal findings, and gives a clear health signal for local work and CI.

## Why It Exists

Instead of stitching together separate tools and output formats, `go-doctor` provides one normalized workflow:

- reliability and maintainability findings in one report
- consistent diagnostic model across analyzers
- repo health score for trend tracking
- baseline and suppression controls for legacy-repo adoption

It is intentionally focused on practical issues that can prevent incidents and regressions.

## Supported Repos

- single-module `go.mod` repos
- multi-module `go.work` repos
- services, libraries, and CLIs

## Quick Start

Detailed quickstart: [docs/quickstart.md](./docs/quickstart.md)

Build from source:

```bash
go build ./cmd/go-doctor
```

Run on the current repo:

```bash
go run ./cmd/go-doctor .
```

Machine-readable output:

```bash
go run ./cmd/go-doctor --format json .
```

SARIF output for GitHub code scanning:

```bash
go run ./cmd/go-doctor --format sarif --output results.sarif .
```

Diff-focused scan (auto base detection, fallback to staged/unstaged):

```bash
go run ./cmd/go-doctor --diff .
```

Diff-focused scan against an explicit base branch:

```bash
go run ./cmd/go-doctor --diff origin/main .
```

In diff mode, `govulncheck` is skipped by default. If you want the middle-ground mode that runs it only for changed modules:

```bash
go run ./cmd/go-doctor --diff --diff-govulncheck changed-modules-only .
```

## Scoring

The health score is local-only and deterministic:

- starts at `100`
- applies severity penalties with per-rule occurrence caps
- applies category multipliers
- rounds once at the final score
- maps labels to: `Excellent` (90-100), `Good` (75-89), `Needs work` (50-74), `Critical` (0-49)

## Rule Discovery

List all available rules and selectors:

```bash
go run ./cmd/go-doctor --list-rules
```

Select rules explicitly:

```bash
go run ./cmd/go-doctor --enable mod/not-tidy,build/mod-readonly-failure .
go run ./cmd/go-doctor --disable fmt/not-gofmt .
```

## GitHub Code Scanning

This repository includes a reusable action at
`./.github/actions/go-doctor` that installs a prebuilt release binary,
runs a SARIF scan, uploads it through `github/codeql-action/upload-sarif`,
and exposes summary outputs.

Minimal workflow usage:

```yaml
permissions:
  contents: read
  security-events: write

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/go-doctor
```

Full end-to-end example:
[docs/examples/go-doctor-sarif-workflow.yml](./docs/examples/go-doctor-sarif-workflow.yml)

CI usage guide: [docs/ci.md](./docs/ci.md)

## Baseline-First Adoption (Recommended)

For mature repos, adopt with a baseline first so legacy findings are visible but do not block rollout.

Local first run (creates baseline file if missing, outside CI):

```bash
go run ./cmd/go-doctor --baseline .go-doctor-baseline.json --fail-on warning .
```

Then commit `.go-doctor-baseline.json` and keep running with the same baseline in CI. Existing findings stay visible as `(suppressed)`, while new findings fail the build based on `--fail-on`.

Full step-by-step onboarding path (including copy-paste config and CI command) is in:
[docs/adoption.md](./docs/adoption.md)

## Inline Suppressions

`go-doctor` supports inline suppressions:

- `// godoctor:ignore <rule> <reason>`
- `// godoctor:ignore-next-line <rule> <reason>`

Examples:

```go
return err.Error() == "EOF" // godoctor:ignore error/string-compare legacy wire-protocol sentinel

// godoctor:ignore-next-line concurrency/ticker-stop test-only fire-and-forget ticker
t := time.NewTicker(time.Second)
```

Suppression policy:

- rule name is required
- reason is required in non-test files
- in `*_test.go` files, reason is optional
- invalid directives are reported as `repo/suppress/invalid`

## Baseline Regeneration

If you intentionally want to refresh baseline entries:

```bash
rm -f .go-doctor-baseline.json
go run ./cmd/go-doctor --baseline .go-doctor-baseline.json .
```

To inspect all findings without applying baseline suppression:

```bash
go run ./cmd/go-doctor --baseline .go-doctor-baseline.json --no-baseline .
```

## Repo Hygiene Rules

Built-in repo-level rules include:

- `mod/not-tidy` (default on, warning)
- `mod/replace-local-path` (default on, warning)
- `build/mod-readonly-failure` (default on, error)
- `fmt/not-gofmt` (default off, info)
- `license/missing` (default off, info)

`mod/not-tidy` and `build/mod-readonly-failure` run checks against temp copies. They do not mutate the checked-out workspace.

## CLI and Config Notes

- CLI flags override config file values.
- Config auto-discovery order: `.go-doctor.yaml`, `.go-doctor.yml`, `.go-doctor.json`.
- `scan.baseline` or `--baseline` enables baseline filtering.
- In CI (`CI=true`), baseline path must exist unless `--no-baseline` is set.
- Third-party analyzer support policy: Go `1.21+`.

Config schema reference: [docs/config-schema.md](./docs/config-schema.md)

Rule catalog: [docs/rules.md](./docs/rules.md)

Performance and profiling notes: [docs/performance.md](./docs/performance.md)

Release gate checklist: [docs/release-gate-v1.md](./docs/release-gate-v1.md)

## Out of Scope

`go-doctor` is not:

- a hosted platform
- an IDE plugin
- an auto-remediation engine
- a replacement for every linter
