# 1. Executive Summary

- **Problem Statement**: `go-doctor` only becomes a strong CI and code-scanning tool once its findings can flow directly into GitHub. This milestone must turn normalized diagnostics into valid SARIF and wrap the binary in an easy-to-adopt GitHub Action.
- **Proposed Solution**: Implement a SARIF `2.1.0` exporter, ship a GitHub Action that installs a release binary and uploads SARIF, and provide an end-to-end CI example for users.
- **Success Criteria**:
  - SARIF output validates against GitHub Advanced Security/code scanning ingestion without schema issues.
  - Each `plugin/rule` is represented as a SARIF rule with correct severity mapping.
  - Artifact paths are relative to repo root.
  - The GitHub Action supports the planned inputs and exposes the planned outputs.
  - A documented end-to-end workflow produces uploaded SARIF from a sample repository.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer shipping the CI integration surface.
  - Go engineer who wants code-scanning annotations in pull requests.
  - CI adopter who wants a minimal action setup with predictable defaults.
- **User Stories**:
  - As a CI adopter, I want a ready-made GitHub Action so I can add `go-doctor` to a workflow quickly.
  - As a Go engineer, I want SARIF findings to appear as code-scanning annotations with accurate file locations.
  - As a maintainer, I want the action to prefer stable release binaries instead of rebuilding the tool in every workflow.
- **Acceptance Criteria**:
  - Implement `--format sarif` and `RenderSARIF(result DiagnoseResult) ([]byte, error)`.
  - Emit SARIF `2.1.0` with one run per invocation.
  - Map severities exactly: `error -> error`, `warning -> warning`, `info -> note`.
  - Include help text and documentation URLs in SARIF rule metadata when available.
  - Ship a GitHub Action under `.github/actions/go-doctor/` or the chosen equivalent path using prebuilt binaries.
  - Action inputs include `directory`, `format`, `fail-on`, `diff`, `config`, and optional `github-token` for future use.
  - Action outputs include `score`, `findings`, `errors`, and `warnings`.
  - Provide an end-to-end workflow example that runs the tool and uploads SARIF via `github/codeql-action/upload-sarif`.
  - The default Action flow is: install release binary, run `go-doctor --format sarif --output results.sarif`, upload SARIF, and only run a second machine-readable pass if explicit outputs require it.
- **Non-Goals**:
  - Posting PR comments in v1.
  - A hosted scoring service.
  - Replacing the CLI with action-only behavior.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. This milestone is a deterministic export and CI packaging layer.
- **Evaluation Strategy**: Validate SARIF structurally in tests, then verify real GitHub ingestion in an end-to-end repository. Success is schema correctness and actionable annotations, not model performance.

# 4. Technical Specifications

- **Architecture Overview**:
  - Keep SARIF rendering in `internal/output/sarif` and expose a stable wrapper via `pkg/godoctor`.
  - Build the exporter from normalized diagnostics so third-party and custom findings share one mapping path.
  - Keep the GitHub Action thin: install binary, run `go-doctor`, upload SARIF, and expose outputs.
- **Integration Points**:
  - Existing `DiagnoseResult` and rule metadata for SARIF rule definitions.
  - GitHub Actions runtime plus `github/codeql-action/upload-sarif`.
  - Release packaging from the hardening milestone for long-term binary distribution.
- **Security & Privacy**:
  - Do not require `github-token` for baseline v1 behavior.
  - Upload only scan results chosen by the user; no extra telemetry or PR comment side effects.
  - Keep paths relative to repo root to avoid leaking machine-specific absolute paths.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Implement the SARIF renderer and validate its schema shape.
  - Add the GitHub Action wrapper with the defined inputs, outputs, and default flow.
  - Verify end-to-end GitHub ingestion with a working example workflow.
- **Technical Risks**:
  - Small SARIF schema mistakes can break ingestion entirely. Add structural validation before manual GitHub testing.
  - Action behavior can drift from CLI defaults. Keep inputs aligned with the main binary flags and documented defaults.
  - Relative path mistakes will make annotations useless. Normalize locations against repo root consistently.
