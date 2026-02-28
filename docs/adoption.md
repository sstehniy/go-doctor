# Baseline-First Adoption Guide

This guide is the recommended rollout path for existing repositories:

1. start with a baseline
2. enforce only net-new findings in CI
3. tighten over time

The flow below is designed to be completed in under 10 minutes.

## 10-Minute Onboarding

Run from the target repository root.

1. Create `.go-doctor.yaml`:

```yaml
scan:
  baseline: .go-doctor-baseline.json
  timeout: 45s
ci:
  failOn: warning
output:
  format: text
rules:
  enable:
    - mod/not-tidy
    - mod/replace-local-path
    - build/mod-readonly-failure
```

2. Generate baseline locally (outside CI):

```bash
go-doctor --config .go-doctor.yaml .
```

3. Commit both files:

- `.go-doctor.yaml`
- `.go-doctor-baseline.json`

4. Add CI command:

```bash
go-doctor --config .go-doctor.yaml .
```

With this setup:

- all findings are still shown in output
- baseline-matched findings are marked suppressed
- only new unsuppressed findings can fail CI (based on `ci.failOn`)

## Local Usage

Normal local run:

```bash
go-doctor --config .go-doctor.yaml .
```

Inspect everything without baseline suppression:

```bash
go-doctor --config .go-doctor.yaml --no-baseline .
```

JSON output for scripting:

```bash
go-doctor --config .go-doctor.yaml --format json .
```

## CI Usage

CI should run with the same config and checked-in baseline:

```bash
go-doctor --config .go-doctor.yaml .
```

`go-doctor` treats `CI=true` as CI mode. In CI mode, if `scan.baseline` points to a missing file, the run fails immediately to prevent silent policy drift.

## Baseline Regeneration Policy

Regenerate baseline when you intentionally accept a new steady-state.

Recommended procedure:

1. Run once without baseline to review all active findings.
2. Fix what you want to fix now.
3. Regenerate baseline.
4. Review and commit baseline changes in the same PR.

Commands:

```bash
go-doctor --config .go-doctor.yaml --no-baseline .
rm -f .go-doctor-baseline.json
go-doctor --config .go-doctor.yaml .
```

Note: baseline generation is local-only. It is not auto-created in CI.

## Suppression Policy

Prefer fixing issues. Use suppressions only for justified exceptions.

Supported directives:

- `// godoctor:ignore <rule> <reason>`
- `// godoctor:ignore-next-line <rule> <reason>`

Examples:

```go
result := err.Error() == "EOF" // godoctor:ignore error/string-compare protocol sentinel compatibility

// godoctor:ignore-next-line net/default-client test fixture uses default client
resp, _ := http.DefaultClient.Do(req)
```

Policy rules:

- suppression must name a rule (or plugin scope)
- reason is required outside test files
- test files (`*_test.go`) may omit reason
- malformed directives produce `suppress/invalid` diagnostics

Keep suppressions narrow and include reasons that are reviewable in code review.

## Tightening Strategy

After baseline rollout is stable:

1. raise strictness (`ci.failOn: warning` -> `ci.failOn: info`) once warning backlog is managed
2. optionally enable informational repo checks like `fmt/not-gofmt` and `license/missing`
3. periodically regenerate baseline to keep accepted debt explicit and current
