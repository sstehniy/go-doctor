# 1. Executive Summary

- **Problem Statement**: A feature-complete scanner is not ready for release until it is fast enough, reliable under failure, well documented, and packaged for the target platforms. The final milestone must make the tool operationally trustworthy.
- **Proposed Solution**: Profile performance, tighten timeout and retry behavior, finish user-facing docs, and ship cross-platform release packaging with smoke-test coverage.
- **Success Criteria**:
  - Medium monorepo scans complete within the target time budget defined during implementation benchmarking.
  - Per-analyzer timeout behavior is enforced and documented.
  - Release artifacts are produced for macOS, Linux, and Windows.
  - Cross-platform smoke tests pass against the release binaries.
  - Documentation covers quickstart, config schema, CI examples, and the rule catalog.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer preparing `go-doctor` for v1 release.
  - Go engineer who expects the tool to feel fast and dependable on real repos.
  - CI adopter who needs predictable runtime and installation guidance across platforms.
- **User Stories**:
  - As a Go engineer, I want scans to finish in a reasonable time on medium-sized repos so the tool fits normal development flow.
  - As a CI adopter, I want explicit timeout and failure behavior so pipeline runtime stays predictable.
  - As a maintainer, I want release packaging and docs complete so users can install and adopt the tool without source-diving.
- **Acceptance Criteria**:
  - Collect runtime profiles against representative fixtures and real-world validation repos.
  - Tune concurrency defaults and analyzer scheduling based on measured bottlenecks rather than guesses.
  - Enforce per-analyzer timeout handling and define retry behavior only where safe and deterministic.
  - Produce release artifacts for macOS, Linux, and Windows.
  - Add cross-platform smoke tests that verify binary startup and a basic scan path.
  - Publish docs for quickstart, config schema, CI usage, and the rule catalog.
  - Unit tests cover config precedence and validation, score math and label boundaries, severity/category weighting, path normalization across OSes, SARIF generation shape, diff file-to-package mapping, rule registry resolution, baseline fingerprint stability, and inline suppression parsing.
  - Repo-level analyzer tests cover temp-copy `go mod tidy` diffing, `replace` directive parsing, `go list -mod=readonly` failure mapping, `gofmt -l` parsing, and `go.work` routing.
  - Adapter tests cover sample outputs, malformed output handling, missing tools from `PATH`, and unsupported Go versions.
  - Integration fixtures include `clean-service`, `basic-api-smells`, `workspace-multi-module`, `vuln-demo`, `legacy-monolith`, `diff-mode-changes`, `http-hardening-demo`, `sql-leaks-demo`, `mod-hygiene-demo`, `timers-leaks-demo`, `http-hardening-clean`, `sql-clean`, and `mod-clean`.
  - Real-world validation records runtime, false positives, noisy analyzers to demote or disable, threshold tuning needs, and baseline adoption friction across 5-10 public Go repos.
  - The v1 release gate requires: stable `text`, `json`, and `sarif`; support for `go.mod` and `go.work`; actionable file/line diagnostics for most findings; support for `--diff`, `--fail-on`, and `--list-rules`; baseline and inline suppression support; at least 18 high-signal custom rules across 8 categories; valid GitHub Action SARIF upload; and complete docs.
- **Non-Goals**:
  - Hosted telemetry dashboards.
  - Bundling third-party analyzers inside the binary.
  - Shipping autofix in v1.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. This milestone is performance, reliability, packaging, and documentation work.
- **Evaluation Strategy**: Use profiling, smoke tests, and real-world validation runs across 5-10 public Go repos. Success is measured by runtime, stability, and installation clarity.

# 4. Technical Specifications

- **Architecture Overview**:
  - Preserve the bounded worker pool model and tune defaults using measured scan data.
  - Keep timeout handling centralized in the app runner so all analyzer types respect one policy.
  - Treat docs and release packaging as product deliverables, not post-release cleanup.
- **Integration Points**:
  - Benchmark and profiling tooling for runtime analysis.
  - Cross-platform build and release automation.
  - Existing fixture and public-repo validation process for regression detection.
  - Release packaging must preserve the CLI binary, the embeddable Go package API, JSON/SARIF compatibility, and the GitHub Action contract established in earlier milestones.
- **Security & Privacy**:
  - Keep scans read-only even under retries or profiling.
  - Document any external tool expectations clearly instead of silently downloading dependencies.
  - Validate release artifacts on each platform to reduce the risk of shipping broken binaries.
  - Supported Go versions are the current stable release and the previous stable release only; local CLI should surface missing-tool guidance instead of auto-installing dependencies.
  - Generated files and `vendor/` are excluded by default, repo-level analyzers must never mutate the checked-out workspace, JSON schema version starts at `1`, and v1 does not include a plugin SDK, autofix, or a hosted service.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Benchmark the current pipeline and identify bottlenecks in analyzer scheduling, package loading, or output rendering.
  - Tune timeout and concurrency behavior, then finish release packaging and smoke-test automation.
  - Complete validation, docs, and release-gate checks so the full `m1` through `m7` scope ships as `v1.0`.
- **Technical Risks**:
  - Analyzer noise can block adoption; keep conservative defaults, demote noisy checks, validate on public repos, and rely on baselines plus suppressions to stage rollout.
  - Toolchain and version drift can break local and CI runs; detect missing tools and unsupported versions early, and keep install guidance explicit instead of bundling vendored binaries in v1.
  - Slow scans in large repos can undermine usability; preserve package scoping, bounded concurrency, per-analyzer timeouts, and expensive-tool skipping in diff mode.
  - Architecture rules can become too opinionated; keep custom layer rules config-driven and default to objective cycle detection when no custom layering is configured.
  - Cross-platform path and process behavior can diverge late. Smoke-test real binaries, not only unit-tested code.
