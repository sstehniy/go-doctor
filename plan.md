# Go Doctor: Native Go CLI + CI Health Scanner

## Summary

Build `go-doctor` as a native Go CLI that scans Go modules/workspaces, runs third-party analyzers plus first-party custom rules, normalizes all findings into one diagnostic model, computes a 0-100 health score, and supports local terminal use plus CI/GitHub code scanning.

Chosen defaults:
- Native Go implementation
- First release = CLI + CI/GitHub Action
- Custom rules included in v1, not deferred
- No autofix in v1
- Human output + JSON + SARIF are first-class

Out of scope for v1:
- Web app / share pages
- Aggressive code rewriting
- IDE plugin
- Hosted scoring API

## Product Goals

Primary user:
- Go engineers reviewing service, library, or CLI repos in local dev and CI

Success criteria:
- One command from repo root returns a score, categorized findings, and actionable suggestions
- CI can fail on configurable severity thresholds
- GitHub can ingest SARIF and show code-scanning annotations
- Tool works on both single-module (`go.mod`) and workspace (`go.work`) repos
- False positive rate stays low enough for default-on CI use

Non-goals:
- Replace all linters
- Enforce team-specific style preferences
- Perform risky automatic rewrites

## v1 Scope

Deliverables:
- `go-doctor` CLI binary
- Go package API for embedding
- Built-in analyzer pipeline
- JSON output contract
- SARIF exporter
- GitHub Action wrapper
- Config file support
- Diff mode for changed packages/files
- Exit code policy for CI
- Snapshot/integration test fixtures

Initial checks:
- Third-party adapters:
  - `go vet`
  - `staticcheck`
  - `govulncheck`
  - selected `golangci-lint` analyzers, but invoked as analyzers, not as the only backend
- First-party custom rules:
  - error handling
  - context propagation
  - concurrency safety
  - package architecture
  - test health
  - performance smells

Coverage expansion:
- Prefer checks that prevent production outages (leaks, timeouts, shutdown, retries, DB misuse) over style-only checks
- Prefer analyzers that can point to a concrete file/line with specific remediation
- Default-on rules must pass three gates:
  - low false positives on fixtures
  - clear fix guidance
  - useful without whole-program taint analysis
- Organize coverage by issue family, even when a family mixes third-party analyzers and first-party rules
- Normalize every family into the same `Diagnostic` model for scoring, output, baseline, and suppressions

High-ROI issue families to add:
- resource lifecycle and cancellation
- HTTP client and server hardening
- SQL/DB correctness and context usage
- time and timers (leaks + busy loops)
- module/repo hygiene and reproducibility
- public API safety (breaking change risk signals)
- logging/exit behavior in libraries

## Architecture

### Repo Layout

Recommended mono-repo shape:
- `cmd/go-doctor/`
- `internal/app/`
- `internal/config/`
- `internal/discovery/`
- `internal/diagnostics/`
- `internal/analyzers/thirdparty/`
- `internal/analyzers/custom/`
- `internal/scoring/`
- `internal/output/text/`
- `internal/output/json/`
- `internal/output/sarif/`
- `internal/diff/`
- `internal/ci/github/`
- `pkg/godoctor/`
- `testdata/fixtures/`
- `.github/actions/go-doctor/` or separate action repo if release flow later requires it

Use `internal/` for implementation details, `pkg/godoctor` only for stable embedding API.

### Execution Flow

1. Resolve target directory.
2. Discover repo shape:
   - `go.work` present or not
   - module roots
   - Go version from `go.mod`
   - package graph
   - VCS diff context if `--diff`
3. Load config and merge CLI overrides.
4. Build analysis plan:
   - repo-level checks
   - package selection
   - file/package filters
   - enabled analyzers
5. Run repo-level and package-level analyzers concurrently with bounded worker pool.
6. Normalize raw outputs into `Diagnostic`.
7. Filter/suppress findings by config.
8. Score by rule severity/category weights.
9. Render:
   - terminal summary
   - optional verbose details
   - optional JSON
   - optional SARIF
10. Set exit code based on `--fail-on`.

### Concurrency Model

- One coordinator goroutine
- Worker pool for analyzers, default `min(runtime.NumCPU(), 6)`
- Run analyzers at repo/package granularity depending on analyzer capability
- Repo-level analyzers may inspect temp copies of modules/workspaces, but never mutate the user workspace
- Global context cancellation on fatal infrastructure failure
- Per-analyzer timeout
- Analyzer failures produce tool-level warnings; they do not crash the scan unless all analyzers fail

## Analyzer Design

### Common Analyzer Interface

Publicly stable inside package API, internal for implementations:

```go
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type AnalyzerTarget struct {
	RepoRoot       string
	ModuleRoots    []string
	PackagePatterns []string
	IncludeFiles   []string
}

type AnalyzerResult struct {
	Diagnostics []Diagnostic
	ToolErrors  []ToolError
	Metadata    map[string]string
}

type Analyzer interface {
	Name() string
	SupportsDiff() bool
	Run(ctx context.Context, target AnalyzerTarget) (AnalyzerResult, error)
}
```

Decision:
- `info` exists in model/output, but score uses only `error` and `warning`
- third-party adapters implement same interface as custom analyzers
- repo-level analyzers use the same interface; they run with empty `PackagePatterns` and anchor diagnostics to files like `go.mod`, `go.sum`, `go.work`, or workflow YAML

### Repo-Level Analyzers

Objective repo health checks that are not naturally package-scoped:

- `mod/not-tidy`
  - default ON, `warning`
  - run `go mod tidy` in a temp module copy and diff `go.mod` / `go.sum`
  - anchor diagnostics to `go.mod:1` (or the module directive) with explicit "run `go mod tidy`" help
- `mod/replace-local-path`
  - default ON, `warning`
  - detect `replace` directives that point at `../relative` or absolute local disk paths
- `build/mod-readonly-failure`
  - default ON, `error`
  - run `go list -mod=readonly ./...` per module to catch CI-breaking sum mutations
- `fmt/not-gofmt`
  - default OFF or `info`
  - run `gofmt -l` without writes and surface affected files
- `license/missing`
  - default OFF
  - only check for presence of a common license file in v1; no full compliance scanning

### Third-Party Adapters

`go vet`:
- Run package-scoped
- Parse standard compiler-style output
- Map to `plugin=govet`
- Severity default `warning`, promote known correctness issues to `error` only if rule map explicitly marks them

`staticcheck`:
- Use structured output if available; otherwise parse text deterministically
- Map categories from check prefix:
  - `SA*` correctness/reliability
  - `ST*` style/maintainability
  - `S*` simplification
  - `QF*` quickfix candidates
- Score only selected high-signal classes by default; style-only findings enabled but lower weight

`govulncheck`:
- Run once per module/workspace selection
- Findings with reachable vulnerabilities => `error`
- Informational vulnerabilities without reachable call path => `warning`
- Normalize package/symbol details into help text

Selected `golangci-lint` analyzers:
- Do not shell out to full `golangci-lint run` as sole backend
- For v1, using `golangci-lint --disable-all --enable ...` is acceptable for analyzers not worth re-implementing yet, as long as output stays deterministic in fixtures
- Initial enabled set:
  - `errcheck`
  - `ineffassign`
  - `bodyclose`
  - `rowserrcheck`
    - correctness, high signal
    - default ON, `warning`; promote to `error` for non-test code with a `rows` loop
  - `sqlclosecheck`
    - correctness/resource, high signal
    - default ON, `error`
  - `exportloopref` (or equivalent loop variable capture analyzer)
    - correctness/concurrency
    - default ON only when repo Go version is `< 1.22`; otherwise OFF or `info`
  - `prealloc`
    - performance, lower signal
    - default OFF (or `info`)
  - `gosec` subset only if noise is acceptable in fixture trials
- Gate noisy analyzers behind config defaults if signal is not stable
- Keep only analyzers that survive fixture trials with low false positives and concrete remediation guidance

### Custom v1 Rules

Implement with `go/ast`, `go/token`, `go/types`, and `golang.org/x/tools/go/packages`.

Categories and initial rules:

Add `Resource` as a first-class category so leak/cancellation issues are easy to understand and tune separately from generic correctness.

Error Handling:
- `error/ignored-return`
  - detect ignored returned `error` values outside explicit discard allowlist
- `error/string-compare`
  - detect direct string comparison on `error.Error()` in branch logic
- `error/fmt-error-without-wrap`
  - detect `fmt.Errorf` losing cause where `%w` is expected in wrapping contexts

Context:
- `context/missing-first-arg`
  - exported funcs doing I/O or RPC-like work without `context.Context` first arg
- `context/background-in-request-path`
  - forbid `context.Background()` / `TODO()` in HTTP/RPC request paths
- `context/not-propagated`
  - child calls use unrelated context when parent context available
- `context/with-timeout-not-canceled` (`warning`)
  - detect `WithTimeout` / `WithDeadline` / `WithCancel` where the returned cancel func is not called, returned, or stored for later use

Concurrency:
- `concurrency/go-routine-leak-risk`
  - goroutine started in loop/request path with no shutdown/cancel linkage
- `concurrency/ticker-not-stopped`
  - `time.NewTicker` without `Stop`
- `concurrency/waitgroup-misuse`
  - `Add` inside goroutine, copied waitgroups, obvious unsafe patterns
- `concurrency/mutex-copy`
  - structs containing mutexes copied by value in suspicious flows

Performance:
- `perf/defer-in-hot-loop`
  - defer inside loops
- `perf/fmt-sprint-simple-concat`
  - `fmt.Sprintf` for simple string concat in non-format cases
- `perf/bytes-buffer-copy`
  - avoid unnecessary `[]byte(string(...))` / `string([]byte(...))` churn
- `perf/json-unmarshal-twice`
  - same raw payload decoded multiple times in same control path

Resource / Net Reliability:
- `net/http-request-without-context` (`warning`)
  - when an in-scope `ctx` exists, flag `http.NewRequest` instead of `http.NewRequestWithContext`
- `net/http-default-client` (`warning`)
  - flag `http.Get` / `http.Post` and `http.DefaultClient` in non-test code
- `net/http-server-no-timeouts` (`warning`)
  - flag `http.ListenAndServe` / `ListenAndServeTLS` and `http.Server` values missing `ReadHeaderTimeout`; also check `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`
- `time/tick-leak` (`error`)
  - flag `time.Tick(...)`; recommend `time.NewTicker` plus `Stop()`
- `time/after-in-loop` (`warning`)
  - flag `time.After(...)` inside loops/select loops where repeated timers are created
- `db/rows-not-closed` (`error`)
  - fallback first-party rule if `sqlclosecheck` is unavailable; detect query result sets missing `defer rows.Close()`
- `db/rows-err-not-checked` (`warning`)
  - fallback first-party rule if `rowserrcheck` is unavailable; detect loops that never check `rows.Err()`
- `db/tx-no-deferred-rollback` (`warning`)
  - flag transactions with `Commit()` but no `defer tx.Rollback()` safety net in the same function
- `io/readall-unbounded` (`warning`, default OFF)
  - flag `io.ReadAll(req.Body)` / `io.ReadAll(resp.Body)` in server paths unless bounded by `io.LimitReader` or `http.MaxBytesReader`

Architecture:
- `arch/cross-layer-import`
  - configurable layer boundaries, default convention:
    - `internal/transport` cannot be imported by `internal/domain`
    - `internal/platform` cannot import `internal/transport`
- `arch/forbidden-package-cycles`
  - package cycles reported as hard errors
- `arch/oversized-package`
  - package exceeds configurable file/function thresholds
- `arch/god-file`
  - single file exceeds configurable statement/function count thresholds

Testing:
- `test/missing-table-driven`
  - disabled by default; not objective enough for v1
- `test/no-assertions`
  - tests with setup and no failure path
- `test/sleep-in-test`
  - raw sleeps in tests without polling/retry helper
- `test/http-handler-no-test`
  - warning when package exposes handlers but has no `_test.go`

Security:
- `sec/math-rand-for-secret`
  - use of `math/rand` in token/password/key-like identifiers
- `sec/insecure-temp-file`
  - unsafe temp file/path patterns
- `sec/exec-user-input`
  - command execution with directly tainted user input in simple cases

API Surface:
- `api/error-string-branching` (`warning`)
  - specialized form of `error/string-compare`; escalate when branching logic depends on `err.Error()` for exported or cross-package error paths
- `api/exported-mutable-global` (`warning`)
  - flag exported mutable globals like maps, slices, or pointers to structs in non-test code
- `api/init-side-effects` (`warning`)
  - flag `init()` that performs I/O, starts goroutines, or registers global handlers outside `main` / `cmd`

Library Safety:
- `lib/os-exit-in-non-main` (`error` for libraries, `warning` for services)
  - flag `os.Exit`, `log.Fatal`, `zap.L().Fatal`, and similar hard exits outside package `main`
- `lib/flag-parse-in-non-main` (`warning`)
  - flag `flag.Parse()` outside `main` / configured `cmd/**` packages

Rule philosophy:
- Objective, high-signal checks only
- Prefer outage-preventing rules over taste/style
- Every rule must include remediation help
- Rules with >10% fixture false positives move behind opt-in config
- Default-on rules must pass:
  - low fixture false positives
  - clear fix guidance
  - useful signal without whole-program taint analysis
- Keep confidence as an internal scoring/gating concept in v1; expose it later only if the JSON envelope needs it without changing the exported Go structs

## Diagnostic Model

Public stable model in `pkg/godoctor`:

```go
type Diagnostic struct {
	FilePath      string   `json:"filePath"`
	Line          int      `json:"line"`
	Column        int      `json:"column"`
	EndLine       int      `json:"endLine,omitempty"`
	EndColumn     int      `json:"endColumn,omitempty"`
	Plugin        string   `json:"plugin"`
	Rule          string   `json:"rule"`
	Severity      Severity `json:"severity"`
	Category      string   `json:"category"`
	Message       string   `json:"message"`
	Help          string   `json:"help,omitempty"`
	Symbol        string   `json:"symbol,omitempty"`
	PackagePath   string   `json:"packagePath,omitempty"`
	ModulePath    string   `json:"modulePath,omitempty"`
	DocumentationURL string `json:"documentationUrl,omitempty"`
	Weight        int      `json:"weight,omitempty"`
}
```

Supporting public types:

```go
type ScoreResult struct {
	Score int    `json:"score"`
	Label string `json:"label"`
}

type ToolError struct {
	Analyzer string `json:"analyzer"`
	Message  string `json:"message"`
}

type ProjectInfo struct {
	RepoRoot        string   `json:"repoRoot"`
	GoVersion       string   `json:"goVersion"`
	ModuleRoots     []string `json:"moduleRoots"`
	WorkspaceFile   string   `json:"workspaceFile,omitempty"`
	PackageCount    int      `json:"packageCount"`
	ScannedPackages []string `json:"scannedPackages"`
}

type DiagnoseResult struct {
	ProjectInfo  ProjectInfo   `json:"projectInfo"`
	Diagnostics  []Diagnostic  `json:"diagnostics"`
	Score        *ScoreResult  `json:"score,omitempty"`
	SkippedTools []string      `json:"skippedTools,omitempty"`
	ToolErrors   []ToolError   `json:"toolErrors,omitempty"`
	ElapsedMS    int64         `json:"elapsedMs"`
}
```

Decision:
- Keep all stable exported types under `pkg/godoctor`
- Internal analyzer raw schemas stay unexported

## Scoring Model

Local-only scoring, no hosted API in v1.

Base:
- Start at 100
- Deduct by unique rule hit, not raw occurrence count, to prevent one issue exploding score
- Add small per-occurrence penalty cap per rule to reflect widespread impact

Default penalties:
- Error rule first hit: `-8`
- Warning rule first hit: `-3`
- Info: `0`
- Additional occurrences of same rule:
  - errors: `-1`, capped at `-5` extra per rule
  - warnings: `-0.5`, rounded at final score, capped at `-3` extra per rule

Category multipliers:
- Security x1.5
- Correctness / Concurrency / Resource x1.25
- Net / HTTP hardening x1.0 by default
- Context / Architecture / API Surface / Library Safety x1.0
- Performance / Testing x0.75
- Style-like third-party findings x0.5

Score labels:
- `90-100` Excellent
- `75-89` Good
- `50-74` Needs work
- `0-49` Critical

Scoring behavior:
- `govulncheck` reachable vuln with fix available can floor score at max 49 unless explicitly ignored
- `build/mod-readonly-failure` can floor score at max 74 (`Needs work`) because it predicts CI breakage
- Complete analyzer failure does not affect score directly, but is surfaced in `ToolErrors`
- Score can be disabled with `--no-score`
- Certain rule families may scale by blast radius heuristics:
  - e.g. `net/http-server-no-timeouts` in `cmd/**` weighs higher than the same finding in `internal/testutil/**`

## CLI Specification

Binary:
- `go-doctor`

Base command:
```bash
go-doctor [directory] [flags]
```

Flags:
- `--config <path>`
- `--format <text|json|sarif>` default `text`
- `--output <path>` write machine output to file; terminal summary still printed unless `--quiet`
- `--verbose`
- `--no-score`
- `--fail-on <error|warning|none>` default `none`
- `--diff [base]`
- `--packages <patterns>` comma-separated package patterns
- `--modules <paths>` comma-separated module roots for workspace repos
- `--timeout <duration>` default `2m`
- `--concurrency <n>`
- `--enable <rule-or-group>` repeatable
- `--disable <rule-or-group>` repeatable
- `--baseline <path>`
- `--no-baseline`
- `--list-rules`
- `--version`
- `--quiet`

Decisions:
- No `--fix` in v1
- `text` is human default
- `json` and `sarif` are non-interactive and deterministic
- `--diff` with no base:
  - use merge-base against default branch if detectable
  - fallback to uncommitted changes
  - if neither available, log warning and run full scan

Exit codes:
- `0`: success; below fail threshold not triggered
- `1`: diagnostics crossed `--fail-on`
- `2`: usage/config error
- `3`: fatal runtime failure (repo unreadable, all analyzers failed, invalid Go toolchain)

## Config Specification

Config file names, precedence:
1. explicit `--config`
2. `.go-doctor.yaml`
3. `.go-doctor.yml`
4. `.go-doctor.json`

Recommended schema:

```yaml
version: 1

scan:
  modules:
    - "."
  packages:
    - "./..."
  exclude:
    files:
      - "**/*_generated.go"
      - "**/mock_*.go"
    directories:
      - "vendor"
      - "third_party"

output:
  format: text
  verbose: false
  score: true

ci:
  failOn: error
  mainPackages:
    - "cmd/**"

rules:
  enable:
    - "error/*"
    - "context/*"
    - "concurrency/*"
  disable:
    - "test/http-handler-no-test"

ignore:
  baseline: ".go-doctor-baseline.json"
  rules:
    - "staticcheck/ST1000"
  files:
    - "internal/legacy/**"

thresholds:
  oversizedPackageFiles: 12
  oversizedPackageFuncs: 60
  godFileFuncs: 15
  godFileStatements: 300

architecture:
  layers:
    - name: domain
      packages: ["internal/domain/**"]
      mayImport: ["internal/domain/**", "internal/platform/**"]
    - name: transport
      packages: ["internal/transport/**"]
      mayImport: ["internal/domain/**", "internal/platform/**"]

analyzers:
  govet: true
  staticcheck: true
  govulncheck:
    enabled: true
    diffMode: skip
  golangci:
    enabled: true
    linters:
      - errcheck
      - ineffassign
      - bodyclose
      - rowserrcheck
      - sqlclosecheck
```

Rules:
- CLI flags override config
- Unknown config keys fail fast with clear message
- Unknown rules in `enable/disable/ignore.rules` fail fast
- Glob patterns use forward-slash normalized paths

Suppressions and adoption controls:
- Support inline suppressions:
  - `// godoctor:ignore <rule> <reason>`
  - `// godoctor:ignore-next-line <rule> <reason>`
- Require a reason string for non-test code by default
- Baseline files store stable fingerprints (`rule + file + range + normalized message`)
- In CI with a baseline present:
  - show all findings
  - fail only on new findings unless `--no-baseline`

## Diff Mode

v1 behavior:
- Determine changed files via Git
- Map changed files to owning packages
- Always run repo-level checks even in diff mode:
  - `mod/not-tidy`
  - `mod/replace-local-path`
  - `build/mod-readonly-failure`
- Run third-party analyzers package-scoped for affected packages only
- Run custom file/package analyzers only on affected packages
- `govulncheck` is repo/module scoped and too expensive to meaningfully diff; in diff mode:
  - skip by default
  - allow `analyzers.govulncheck.diffMode: changed-modules-only` as the middle-ground option when changed files can be mapped to workspace modules
  - print `SkippedTools` entry
  - full-repo diff-mode override can wait until later; v1 only adds the `changed-modules-only` middle ground

Git strategy:
- If base branch supplied, use `git merge-base HEAD <base>`
- Else detect upstream default branch from remote HEAD
- Else compare unstaged/staged changes
- Exclude deleted files from file-level analysis, but use deletions for package recalculation when needed

## Output Contracts

### Text Output

Sections:
- banner
- project summary
- score gauge
- category summary
- grouped findings by `plugin/rule`
- optional affected file lines in verbose mode
- tool warnings / skipped analyzers
- remediation footer

Text grouping:
- sort by severity desc, then category, then rule
- first line = message and count
- second line = help text
- verbose = file paths + line numbers

### JSON Output

Structured `DiagnoseResult`, stable for CI consumers.

Versioning:
- add top-level `schemaVersion: 1` to JSON envelope
- allow supplemental envelope metadata for baseline filtering and suppressions without changing exported Go structs
- breaking output changes require schema version bump

### SARIF Output

- SARIF `2.1.0`
- one run per invocation
- each `plugin/rule` becomes a SARIF rule
- map:
  - error -> `error`
  - warning -> `warning`
  - info -> `note`
- include help text and docs URL
- include artifact locations relative to repo root

## Embeddable Go API

Public package:
- `pkg/godoctor`

Stable API:

```go
type Options struct {
	ConfigPath   string
	Format       string
	Verbose      bool
	ScoreEnabled bool
	FailOn       string
	DiffBase     string
	PackagePatterns []string
	ModuleRoots  []string
	Timeout      time.Duration
	Concurrency  int
	EnabledRules []string
	DisabledRules []string
}

func Diagnose(ctx context.Context, directory string, options Options) (DiagnoseResult, error)
func ListRules() []RuleDescriptor
func RenderSARIF(result DiagnoseResult) ([]byte, error)
```

Additional public type:

```go
type RuleDescriptor struct {
	Plugin      string   `json:"plugin"`
	Rule        string   `json:"rule"`
	Category    string   `json:"category"`
	DefaultOn   bool     `json:"defaultOn"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Help        string   `json:"help"`
}
```

Decision:
- keep API minimal; no public analyzer plugin system in v1
- plugin extensibility can be designed later without locking unstable hooks now

## CI / GitHub Action Plan

GitHub Action:
- Use prebuilt `go-doctor` release binaries
- Inputs:
  - `directory` default `.`
  - `format` default `sarif`
  - `fail-on` default `error`
  - `diff` optional branch
  - `config` optional
  - `github-token` optional, only if PR summary posting added later
- Outputs:
  - `score`
  - `findings`
  - `errors`
  - `warnings`

Action workflow:
1. install binary
2. run `go-doctor --format sarif --output results.sarif`
3. upload SARIF with `github/codeql-action/upload-sarif`
4. optionally run second `json` pass only if outputs are needed; better: generate JSON in-process if action wrapper calls library directly

Decision:
- v1 GitHub Action should not post PR comments
- use SARIF upload as the primary GitHub UX

## Implementation Milestones

### Milestone 1: Core Skeleton
- repo bootstrap
- CLI flags
- config loader
- project discovery
- diagnostic types
- text/json output
- exit code policy

Acceptance:
- binary runs on single-module repo and returns empty healthy result

### Milestone 2: Third-Party Adapters
- `go vet`
- `staticcheck`
- `govulncheck`
- selected `golangci` analyzers
- parser + normalization layer
- tool error reporting

Acceptance:
- fixture repos produce deterministic diagnostics from each tool
- analyzer failures do not crash entire scan

### Milestone 3: Custom Rules v1
- implement first batch:
  - error handling
  - context
  - concurrency
  - resource / net reliability
  - security
  - API surface
  - library safety
  - architecture
  - performance
- add rule registry + rule docs
- add config-based enable/disable

Acceptance:
- each rule has at least one positive fixture and one negative fixture
- false-positive review performed on 3 real-world sample repos before default-on

### Milestone 3.5: Repo Hygiene + Adoption Controls
- repo-level analyzers:
  - mod tidy temp-copy diff
  - readonly build check
  - local replace detection
  - optional `gofmt` check
- baseline:
  - generate baseline
  - apply baseline filtering in CI mode
- inline suppressions:
  - `ignore`
  - `ignore-next-line`
  - reason enforcement
- documentation:
  - adoption guide: start with baseline, then tighten

Acceptance:
- a legacy repo can be onboarded with a baseline in under 10 minutes
- CI fails only on newly introduced issues when a baseline is present

### Milestone 4: Scoring + Diff
- local score engine
- score labels
- diff file discovery
- package narrowing
- repo-level checks still run in diff mode
- skipped-tools reporting

Acceptance:
- diff mode only scans affected packages
- score identical between repeated runs with same inputs

### Milestone 5: SARIF + GitHub Action
- SARIF exporter
- action definition
- end-to-end CI example

Acceptance:
- GitHub Advanced Security/code scanning ingests SARIF without schema issues

### Milestone 6: Hardening
- perf profiling
- timeout/retry policy
- docs
- release packaging for macOS/Linux/Windows

Acceptance:
- medium monorepo scan finishes in target time budget
- cross-platform smoke tests pass

## Testing Plan

### Unit Tests
- config precedence and validation
- score math and label boundaries
- severity/category weighting
- path normalization across OSes
- SARIF generation schema shape
- diff file to package mapping
- rule registry enable/disable resolution
- baseline fingerprint stability
- inline suppression parsing and reason enforcement

### Repo-Level Analyzer Tests
- temp-copy `go mod tidy` diffing without workspace mutation
- `replace` directive parsing for relative and absolute paths
- `go list -mod=readonly` failure mapping
- `gofmt -l` parsing and file anchoring
- module/workspace routing when `go.work` is present

### Analyzer Adapter Tests
- parse sample outputs for:
  - `go vet`
  - `staticcheck`
  - `govulncheck`
  - selected `golangci` linters
- malformed output handling
- tool missing from PATH
- unsupported Go version

### Custom Rule Tests
Per rule:
- positive fixture: emits expected diagnostic
- negative fixture: emits none
- line/column accuracy
- help text present
- ignores generated files if configured

### Integration Fixtures
Create fixture repos:
- `clean-service`
- `basic-api-smells`
- `workspace-multi-module`
- `vuln-demo`
- `legacy-monolith`
- `diff-mode-changes`
- `http-hardening-demo`
- `sql-leaks-demo`
- `mod-hygiene-demo`
- `timers-leaks-demo`
- `http-hardening-clean`
- `sql-clean`
- `mod-clean`

For each fixture validate:
- text snapshot
- JSON snapshot
- SARIF structural validation
- score snapshot
- exit code under each `--fail-on`

### Real-World Validation
Before v1 release, run read-only evaluations against 5-10 public Go repos:
- small library
- HTTP service
- CLI tool
- mono-repo with `go.work`
- concurrency-heavy system

Record:
- runtime
- false positives
- noisy analyzers to demote/disable
- rules needing threshold tuning
- baseline adoption friction

## Acceptance Criteria

The v1 implementation is done when:
- `go-doctor .` works on macOS/Linux/Windows with supported Go versions
- supports `go.mod` and `go.work`
- returns stable `text`, `json`, and `sarif`
- emits actionable diagnostics with file/line for most findings
- supports `--diff`, `--fail-on`, `--list-rules`
- supports repo-level analyzers for module hygiene and reproducibility
- supports baseline + inline suppressions for gradual adoption
- includes at least 18 high-signal custom rules across 8 categories
- GitHub Action uploads valid SARIF
- docs include quickstart, config schema, CI examples, and rule catalog

## Risks and Mitigations

High noise from security/static analyzers:
- keep conservative default set
- demote noisy checks to warning or opt-in
- validate on public repos before default-on
- provide baseline + inline suppressions so teams can adopt without hiding new regressions

Toolchain/version drift:
- detect missing tools and unsupported versions early
- ship clear install docs
- optionally bundle version checks, not vendored binaries, in v1

Slow scans in large repos:
- package scoping
- bounded concurrency
- skip expensive analyzers in diff mode
- per-analyzer timeout

Architecture rules too opinionated:
- make layer rules config-driven
- keep only cycle detection default-on if custom layers not configured

## Assumptions and Defaults

- Supported Go versions in v1: current stable and previous stable only
- `staticcheck` and `govulncheck` are expected installed or bootstrap-installed by the action; local CLI surfaces missing-tool guidance instead of auto-installing
- Generated files are excluded by default using standard patterns (`*.pb.go`, `*_mock.go`, `zz_generated.*`, configurable)
- Vendor directories are excluded by default
- Repo-level analyzers never mutate the checked-out workspace; temp copies only
- No plugin SDK for third parties in v1
- No autofix in v1
- No hosted service/website in v1
- JSON schema version starts at `1`
- Unresolved questions: none
