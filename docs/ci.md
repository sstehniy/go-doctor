# CI Usage

## CLI-only CI step

```yaml
- name: go-doctor
  run: |
    go build -o go-doctor ./cmd/go-doctor
    ./go-doctor --format sarif --output results.sarif --fail-on warning .
```

## GitHub code scanning upload

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
      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: ${{ steps.godoctor.outputs.sarif-path }}
```

A full runnable example is in [examples/go-doctor-sarif-workflow.yml](./examples/go-doctor-sarif-workflow.yml).

## Baseline in CI

- Commit baseline file (for example `.go-doctor-baseline.json`).
- Keep `scan.baseline` or `--baseline` enabled in CI.
- In CI (`CI=true`), a missing baseline path fails fast.

## Exit policy

- `--fail-on none`: never fails on findings.
- `--fail-on info`: fails on any unsuppressed finding.
- `--fail-on warning`: fails on warning/error unsuppressed findings.
- `--fail-on error`: fails on error unsuppressed findings.
