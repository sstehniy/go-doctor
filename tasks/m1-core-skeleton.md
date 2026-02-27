# 1. Executive Summary

Status: Done

- **Problem Statement**: `go-doctor` does not yet have a runnable foundation, so none of the planned analyzer or CI work can ship safely. The first milestone must establish the CLI, project model, config loading, and stable output contracts that every later milestone depends on.
- **Proposed Solution**: Build the repository skeleton, core execution pipeline, stable diagnostic/result types, and human plus JSON output so the tool can complete a no-findings scan on a simple Go repo. Treat this milestone as the contract-setting layer for the rest of v1.
- **Success Criteria**:
  - `go-doctor .` runs successfully on a single-module fixture and exits `0` with an empty healthy result.
  - The binary supports the full v1 CLI contract for target selection, config, output mode, scoring, failure thresholds, diff mode, package/module narrowing, rule toggles, baseline handling, and discovery utilities.
  - Config precedence works in this order: `--config`, `.go-doctor.yaml`, `.go-doctor.yml`, `.go-doctor.json`.
  - `pkg/godoctor` exposes stable `Diagnostic`, `DiagnoseResult`, `ProjectInfo`, `ScoreResult`, and `ToolError` types.
  - The repository supports both single-module (`go.mod`) and workspace (`go.work`) discovery paths.
  - `text` and `json` output paths are deterministic in snapshot tests.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer building the initial `go-doctor` codebase.
  - Go engineer running the tool locally from repo root.
  - CI adopter who needs stable exit codes and machine-readable output.
- **User Stories**:
  - As a maintainer, I want the repo organized into stable packages so later analyzer work lands cleanly.
  - As a Go engineer, I want `go-doctor .` to run without extra setup on a healthy repo so I can trust the default experience.
  - As a CI adopter, I want deterministic JSON output and clear exit codes so I can automate around the tool.
- **Acceptance Criteria**:
  - Repo layout exists for `cmd/go-doctor`, `internal/app`, `internal/config`, `internal/discovery`, `internal/diagnostics`, `internal/analyzers/thirdparty`, `internal/analyzers/custom`, `internal/scoring`, `internal/output/text`, `internal/output/json`, `internal/output/sarif`, `internal/diff`, `internal/ci/github`, `pkg/godoctor`, and `testdata/fixtures`.
  - CLI parses the directory arg and the v1 flag contract: `--config`, `--format`, `--output`, `--verbose`, `--no-score`, `--fail-on`, `--diff`, `--packages`, `--modules`, `--timeout`, `--concurrency`, `--enable`, `--disable`, `--baseline`, `--no-baseline`, `--list-rules`, `--version`, and `--quiet`.
  - Project discovery detects repo root, `go.mod` versus `go.work`, module roots, Go version, and package count.
  - The execution pipeline resolves target -> loads config -> builds an empty analysis plan -> renders output -> sets exit code.
  - Config parsing accepts `scan`, `output`, `ci`, `rules`, `ignore`, `thresholds`, `architecture`, and `analyzers` sections, with unknown keys and unknown rule names failing fast.
  - Exit codes follow the v1 contract: `0` success, `1` threshold breach, `2` usage/config error, `3` fatal runtime failure.
  - JSON output includes `schemaVersion: 1` and a stable `DiagnoseResult` envelope.
  - The public API surface is reserved as `Options`, `Diagnose(...)`, `ListRules()`, and `RenderSARIF(...)`, even if some implementations land in later milestones.
- **Non-Goals**:
  - Running real third-party analyzers.
  - Implementing custom rule logic.
  - SARIF generation or GitHub Action support.
  - Diff mode, baselines, or inline suppressions.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. This milestone is a deterministic CLI and schema foundation; no AI subsystem is required.
- **Evaluation Strategy**: Validate with unit and snapshot tests only. A passing result is deterministic output and stable contracts, not model quality.

# 4. Technical Specifications

- **Architecture Overview**:
  - Create the package boundaries for `cmd/go-doctor`, `internal/app`, `internal/config`, `internal/discovery`, `internal/diagnostics`, `internal/analyzers/thirdparty`, `internal/analyzers/custom`, `internal/scoring`, `internal/output/text`, `internal/output/json`, `internal/output/sarif`, `internal/diff`, `internal/ci/github`, `pkg/godoctor`, and `testdata/fixtures`.
  - Implement a top-level app runner in `internal/app` that owns argument parsing, config loading, discovery, and output rendering.
  - Keep exported stable types in `pkg/godoctor`; keep internal orchestration and config parsing under `internal/`.
  - Define an empty analyzer pipeline seam now so later milestones plug into the same execution flow without reshaping public types.
  - Lock the execution flow as: resolve directory, discover repo shape, load config and CLI overrides, build the analysis plan, run analyzers with bounded concurrency, normalize diagnostics, apply filtering and suppressions, score, render, then set exit code.
  - Lock the concurrency model as one coordinator plus a bounded worker pool with default width `min(runtime.NumCPU(), 6)`, per-analyzer timeouts, and global cancellation on fatal infrastructure failure.
- **Integration Points**:
  - Local filesystem: discover `.go-doctor.*`, `go.mod`, `go.work`, and package roots.
  - Go toolchain metadata: read Go version from module files without mutating the workspace.
  - Output writers: terminal stdout plus optional file writes for JSON output.
  - Text output must support the stable section order: banner, project summary, score gauge, category summary, grouped findings, optional verbose file lines, tool warnings/skipped analyzers, and remediation footer.
  - Stable exported data models must cover location fields, `plugin`, `rule`, `severity`, `category`, message/help metadata, optional symbol/package/module/docs URL, optional `Weight`, project info, score, skipped tools, tool errors, and elapsed time.
  - Config precedence must be CLI over config file; path globs must use forward-slash normalized matching; unknown config keys, unknown enabled/disabled rules, and unknown ignored rules fail fast.
  - The config contract includes defaults for `output.format=text`, `output.score=true`, `ci.failOn=error`, baseline path support, threshold tuning fields, layer definitions, and analyzer toggles.
  - The stable API `Options` contract must include config path, format, verbose mode, score enablement, fail mode, diff base, package patterns, module roots, timeout, concurrency, enabled rules, and disabled rules.
  - JSON output may add envelope metadata for baseline filtering and suppressions without changing exported Go structs; breaking JSON changes require a schema version bump.
- **Security & Privacy**:
  - Read-only filesystem behavior only.
  - No network access or telemetry.
  - Normalize paths consistently across macOS, Linux, and Windows for predictable output and future SARIF compatibility.
  - Treat local terminal output plus JSON and future SARIF as first-class outputs; no autofix behavior is part of v1.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Scaffold packages and wire the CLI entrypoint.
  - Implement config loading, discovery, stable public types, and deterministic text/JSON output.
  - Lock exit codes, API contracts, and snapshot coverage before later milestones build on top.
- **Technical Risks**:
  - If public structs move after this milestone, later milestones will create avoidable breaking churn. Freeze exported types now.
  - If config precedence is ambiguous, adoption work in CI will be harder later. Fail fast on unknown keys early.
  - If path normalization is inconsistent, cross-platform snapshots and SARIF output will become brittle in later milestones.
