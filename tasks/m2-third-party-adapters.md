# 1. Executive Summary

Status: Done

- **Problem Statement**: The core CLI can exist without analyzers, but it delivers no real health signal until it can ingest trusted third-party tools. The second milestone must turn external tool output into normalized diagnostics without making scans flaky.
- **Proposed Solution**: Add adapters for `go vet`, `staticcheck`, `govulncheck`, and a controlled subset of `golangci-lint` analyzers, then normalize all findings into the shared `Diagnostic` model with tool error handling.
- **Success Criteria**:
  - Fixture repos produce deterministic normalized diagnostics for each supported third-party tool.
  - Adapter failures surface as `ToolErrors` and do not crash the whole scan unless all analyzers fail.
  - `go vet`, `staticcheck`, and `golangci` adapters run package-scoped; `govulncheck` runs module/workspace-scoped.
  - Severity, plugin, rule, and category mappings are explicit and stable across all adapters.
  - Missing tools from `PATH` are reported with actionable install guidance.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer integrating external analyzers.
  - Go engineer expecting familiar tools consolidated into one command.
  - CI adopter who needs resilient scans even when one tool is unavailable.
- **User Stories**:
  - As a maintainer, I want a common adapter interface so external tools can plug in consistently.
  - As a Go engineer, I want one `go-doctor` run to aggregate multiple analyzers into a single result model.
  - As a CI adopter, I want partial tool failure to degrade gracefully instead of hiding all results.
- **Acceptance Criteria**:
  - Implement the shared analyzer interface with `Name()`, `SupportsDiff()`, and `Run(...)`, plus stable internal target/result contracts for repo root, module roots, package patterns, include-files, diagnostics, tool errors, and analyzer metadata.
  - Add third-party adapter packages under `internal/analyzers/thirdparty`.
  - `go vet` output is parsed from compiler-style lines, mapped to `plugin=govet`, and defaults to `warning` severity unless a rule map explicitly promotes a known correctness issue to `error`.
  - `staticcheck` uses structured output when available, otherwise a deterministic text parser, and maps `SA*` to correctness/reliability, `ST*` to style/maintainability, `S*` to simplification, and `QF*` to quick-fix candidates.
  - `govulncheck` maps reachable findings to `error` and informational findings to `warning`.
  - `golangci` integration only enables the approved initial linter set: `errcheck`, `ineffassign`, `bodyclose`, `rowserrcheck`, `sqlclosecheck`, `exportloopref` (default ON only for Go `< 1.22`), and `prealloc` only behind low-signal defaults; `gosec` may only be included as a conservative subset after fixture trials.
  - `rowserrcheck` defaults to `warning` and may be promoted to `error` for non-test code with a `rows` loop; `sqlclosecheck` defaults to `error`.
  - Normalization preserves file, line, rule, plugin, severity, category, message, and remediation help text where available.
- **Non-Goals**:
  - Implementing first-party AST rules.
  - Scoring logic beyond preserving `Weight` metadata if needed.
  - Expanding `golangci-lint` into a broad "enable everything" wrapper.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. These are deterministic static analysis integrations, not AI-generated findings.
- **Evaluation Strategy**: Use adapter fixtures with saved tool outputs and golden normalized results. Measure determinism, parse coverage, and failure handling rather than model accuracy.

# 4. Technical Specifications

- **Architecture Overview**:
  - Keep all adapters behind the shared analyzer interface so the app runner can schedule them through the same bounded worker pool.
  - Implement a normalization layer that converts tool-specific findings into the stable `Diagnostic` model before any output or scoring logic runs.
  - Store tool-specific parse helpers internally; never leak raw tool schemas into `pkg/godoctor`.
- **Integration Points**:
  - Shell out to installed binaries: `go`, `staticcheck`, `govulncheck`, and `golangci-lint`.
  - Respect module/workspace discovery from Milestone 1 when determining package or module scope.
  - Reuse the shared output pipeline so tool findings show up in both text and JSON without per-tool formatting branches.
  - Keep only analyzers that pass fixture trials with low false positives and clear remediation guidance.
- **Security & Privacy**:
  - Execute analyzers read-only against the checked-out repo.
  - Do not auto-install missing binaries in the local CLI path; report guidance instead.
  - Bound runtime with per-analyzer timeouts and context cancellation to avoid hung CI jobs.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Implement the analyzer interface, command execution wrapper, and normalization scaffolding.
  - Wire `go vet`, `staticcheck`, `govulncheck`, and the approved `golangci` subset.
  - Finish fixture coverage, deterministic parsing, and graceful tool error reporting.
- **Technical Risks**:
  - Output drift between tool versions can break parsers. Favor structured output where possible and fixture-test text parsing aggressively.
  - Security and style analyzers can be noisy. Keep defaults conservative and gate noisy rules behind config.
  - If adapter metadata is inconsistent, scoring and SARIF work will become unstable later. Normalize rule IDs and severities now.
