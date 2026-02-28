# Performance and Profiling

This milestone adds repeatable profiling and benchmarking hooks for scan runtime hardening.

## Benchmark command

```bash
GOCACHE=$(mktemp -d) go test ./pkg/godoctor -run '^$' -bench HardeningFixtures -benchmem
```

Sample run (2026-02-28, Apple M1, Go test bench):

- `basic-api-smells`: `~245ms/op`
- `mod-hygiene-demo`: `~0.28ms/op`
- `workspace-multi-module`: `~0.16ms/op`

## Runtime profile command

```bash
scripts/profile.sh
```

The script captures CPU and memory profiles for representative integration fixtures and writes output under `./artifacts/profiles/`.

## Timeout and retry hardening

Runtime policy:

- global timeout is configured by `--timeout`/`scan.timeout`
- each analyzer run executes under that timeout budget
- timeout is enforced at runner level, not best-effort parser level
- deterministic retry behavior:
  - max `2` attempts
  - retry only on timeout
  - retry only for read-only built-in analyzers

## Concurrency default

`scan.concurrency` defaults to `min(runtime.NumCPU(), 6)`.

This preserves bounded parallelism for medium monorepos while avoiding unbounded process fan-out in CI.
