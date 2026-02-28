# Rule Catalog

## Repo Hygiene Rules

- `mod/not-tidy` (default on, warning)
- `mod/replace-local-path` (default on, warning)
- `build/mod-readonly-failure` (default on, error)
- `fmt/not-gofmt` (default off, info)
- `license/missing` (default off, info)

## Third-party Adapters and Rules

Analyzer rules:

- `govet`
- `staticcheck`
- `govulncheck`
- `golangci-lint`

`golangci-lint` linter rules:

- `errcheck`
- `ineffassign`
- `bodyclose`
- `rowserrcheck`
- `sqlclosecheck`
- `exportloopref` (enabled by default only for Go `< 1.22`)
- `prealloc` (supported opt-in)

## Custom Rules

### API Surface

- `api/error-string-branching`
- `api/exported-mutable-global`
- `api/init-side-effects`

### Architecture

- `arch/cross-layer-import`
- `arch/forbidden-package-cycles`
- `arch/god-file`
- `arch/oversized-package`

### Concurrency

- `concurrency/go-routine-leak-risk`
- `concurrency/mutex-copy`
- `concurrency/ticker-not-stopped`
- `concurrency/waitgroup-misuse`

### Context

- `context/background-in-request-path`
- `context/missing-first-arg`
- `context/not-propagated`
- `context/with-timeout-not-canceled`

### Correctness

- `error/fmt-error-without-wrap`
- `error/ignored-return`
- `error/string-compare`

### Data/Resource Safety

- `db/rows-err-not-checked`
- `db/rows-not-closed`
- `db/tx-no-deferred-rollback`
- `io/readall-unbounded`
- `time/after-in-loop`
- `time/tick-leak`

### Library Safety

- `lib/flag-parse-in-non-main`
- `lib/os-exit-in-non-main`

### Networking

- `net/http-default-client`
- `net/http-request-without-context`
- `net/http-server-no-timeouts`

### Performance

- `perf/bytes-buffer-copy`
- `perf/defer-in-hot-loop`
- `perf/fmt-sprint-simple-concat`
- `perf/json-unmarshal-twice`

### Security

- `sec/exec-user-input`
- `sec/insecure-temp-file`
- `sec/math-rand-for-secret`

### Testing

- `test/http-handler-no-test`
- `test/missing-table-driven`
- `test/no-assertions`
- `test/sleep-in-test`

## List all rules from the binary

```bash
go-doctor --list-rules
```
