# 1. Executive Summary

- **Problem Statement**: Raw findings are useful, but the product vision depends on quick triage and fast feedback in active development. This milestone must turn diagnostics into a stable health score and make scans narrower in diff-driven workflows.
- **Proposed Solution**: Implement the local score engine, score labels, Git-based diff discovery, package narrowing, and explicit skipped-tool reporting while preserving correctness for repo-level checks.
- **Success Criteria**:
  - The score starts at `100` and applies fixed severity, occurrence-cap, and category-multiplier rules deterministically.
  - Score labels match the exact ranges: `Excellent`, `Good`, `Needs work`, and `Critical`.
  - `--diff` narrows package-scoped analysis to affected packages while repo-level checks still run every time.
  - Repeated runs with the same repo state produce identical scores.
  - Diff mode reports skipped expensive tools, including default `govulncheck` behavior.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer implementing triage and fast-path scanning.
  - Go engineer who wants a quick health snapshot while iterating.
  - CI adopter who wants changed-code feedback instead of full-repo latency on every PR.
- **User Stories**:
  - As a Go engineer, I want one score that summarizes the repo’s health without hiding the underlying findings.
  - As a CI adopter, I want diff mode to analyze only affected packages so pull request checks stay fast.
  - As a maintainer, I want skipped or downgraded analyzer behavior to be explicit so users trust the result.
- **Acceptance Criteria**:
  - Implement the exact score math: first `error` hit `-8`, first `warning` hit `-3`, `info` `0`; repeated hits of the same rule apply `-1` per additional error up to `-5` extra and `-0.5` per additional warning up to `-3` extra, with rounding at final score.
  - Apply category multipliers exactly: Security `x1.5`; Correctness, Concurrency, and Resource `x1.25`; Net/HTTP hardening, Context, Architecture, API Surface, and Library Safety `x1.0`; Performance and Testing `x0.75`; style-like third-party findings `x0.5`.
  - Apply score labels exactly: `90-100` Excellent, `75-89` Good, `50-74` Needs work, `0-49` Critical.
  - Support score floors for reachable `govulncheck` findings and `build/mod-readonly-failure`.
  - `--no-score` disables score calculation cleanly.
  - `--diff` with an explicit base uses `git merge-base HEAD <base>`.
  - `--diff` without a base attempts remote default branch detection, then falls back to staged/unstaged changes, and if neither is available logs a warning and runs a full scan.
  - Diff mode always runs `mod/not-tidy`, `mod/replace-local-path`, and `build/mod-readonly-failure` even when package analysis is narrowed.
  - Deleted files are excluded from file-level analysis but still influence package recalculation when needed.
  - `govulncheck` is skipped by default in diff mode, with support for `changed-modules-only` as the allowed middle-ground option.
- **Non-Goals**:
  - Hosted or remote scoring.
  - Full-repo "smart diff" heuristics beyond the documented Git strategy.
  - Changing analyzer severity semantics to fit a score.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. This milestone is deterministic scoring and Git-based scope reduction.
- **Evaluation Strategy**: Use unit tests for score math and fixture repos for diff-to-package mapping. Success means deterministic scores, stable labels, and predictable package selection under multiple Git states.

# 4. Technical Specifications

- **Architecture Overview**:
  - Run scoring after normalization and suppression filtering, but before final rendering.
  - Keep the score engine in `internal/scoring` and expose only `ScoreResult` in `pkg/godoctor`.
  - Implement diff discovery in `internal/diff`, then feed narrowed package/file targets into the shared analyzer execution plan.
  - Preserve repo-level analyzers outside package narrowing so hygiene checks always run.
- **Integration Points**:
  - Git CLI for merge-base, default branch detection, and changed-file discovery.
  - Existing analyzer registry to respect `SupportsDiff()` and emit `SkippedTools`.
  - Output renderers so score and skipped tools appear consistently in text, JSON, and later SARIF envelopes as appropriate.
  - Score remains local-only and does not depend on any hosted scoring API in v1.
- **Security & Privacy**:
  - Read Git state only; do not rewrite branches, index, or working tree state.
  - Make skipped-tool behavior visible so fast-mode scans do not imply hidden completeness.
  - Keep scoring local-only with no external service or uploaded telemetry.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Implement the score engine and lock score math, labels, and floors with tests.
  - Add Git diff discovery and package narrowing for package-scoped analyzers.
  - Finalize skipped-tool reporting and conservative fallbacks for ambiguous Git states.
- **Technical Risks**:
  - Score churn from unstable rule IDs or category mapping will undermine trust. Reuse frozen metadata from earlier milestones.
  - Incorrect diff-to-package mapping can hide regressions. Favor conservative expansion when ownership is ambiguous.
  - Git edge cases in detached HEAD or shallow clones can break CI. Keep fallbacks explicit and well tested.
