# 1. Executive Summary

- **Problem Statement**: Even accurate diagnostics can be difficult to adopt in mature repositories without repo-level hygiene checks and a path for gradual rollout. This milestone must make `go-doctor` useful on legacy repos without forcing teams to fix everything at once.
- **Proposed Solution**: Add repo-level analyzers for module hygiene and reproducibility, then layer in baselines, inline suppressions, and onboarding documentation so teams can adopt the tool incrementally.
- **Success Criteria**:
  - Repo-level analyzers cover `go mod tidy` drift, readonly build failures, local replace directives, and optional `gofmt` status.
  - A baseline can be generated and later applied so CI fails only on newly introduced findings.
  - Inline suppressions support both `ignore` and `ignore-next-line`, with reason enforcement in non-test code.
  - A legacy repo can complete baseline onboarding in under 10 minutes.
  - Repo-level analyzers never mutate the checked-out workspace.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer shaping adoption controls.
  - Go engineer onboarding `go-doctor` to an existing service.
  - CI adopter who needs gradual rollout instead of a day-one red build.
- **User Stories**:
  - As a Go engineer, I want repo-level checks to catch module and formatting drift before CI surprises me.
  - As a CI adopter, I want a baseline so legacy findings do not block rollout while new regressions still fail the build.
  - As a maintainer, I want suppressions to be explicit and documented so teams can justify exceptions instead of silently hiding problems.
- **Acceptance Criteria**:
  - Implement repo-level analyzers for `mod/not-tidy` (default ON, `warning`), `mod/replace-local-path` (default ON, `warning`), `build/mod-readonly-failure` (default ON, `error`), `fmt/not-gofmt` (default OFF or `info`), and `license/missing` only as an optional presence check in v1.
  - `mod/not-tidy` runs in a temp copy, diffs `go.mod` and `go.sum`, and anchors findings to module files without changing the workspace.
  - Baseline files store stable fingerprints derived from `rule + file + range + normalized message`.
  - In CI mode with a baseline present, all findings are shown but only new findings trip `--fail-on`, unless `--no-baseline` is set.
  - Inline comments `// godoctor:ignore <rule> <reason>` and `// godoctor:ignore-next-line <rule> <reason>` are parsed and enforced.
  - Documentation explains the recommended adoption path: baseline first, then tighten.
- **Non-Goals**:
  - Full license compliance scanning.
  - Hidden suppressions without explicit rule names.
  - Auto-remediation of module or formatting issues.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. These are deterministic repo checks and suppression workflows.
- **Evaluation Strategy**: Validate with fixture repos that exercise baseline generation, baseline filtering, suppression parsing, and temp-copy module checks. Success is measured by stable fingerprints and correct fail/no-fail behavior in CI mode.

# 4. Technical Specifications

- **Architecture Overview**:
  - Reuse the same analyzer interface for repo-level checks, with empty `PackagePatterns` and file-anchored diagnostics.
  - Add a baseline filtering stage after normalization and before scoring/output so all downstream consumers see consistent results.
  - Parse inline suppressions during source inspection and apply them through the same filtering pipeline as baselines.
- **Integration Points**:
  - Filesystem temp directories for `go mod tidy` validation.
  - Config keys for baseline path, rule ignores, file ignores, and suppression behavior.
  - CI mode and exit code policy from the core app runner.
  - Repo-level analyzers must anchor findings to concrete files such as `go.mod`, `go.sum`, `go.work`, or workflow YAML rather than package-only results.
- **Security & Privacy**:
  - Never mutate the checked-out repo; all module rewrites happen in temp copies only.
  - Require human-readable reasons for suppressions in non-test code to preserve reviewability.
  - Keep baseline fingerprints stable and deterministic so teams can review them in version control.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Implement repo-level analyzers and anchor their diagnostics correctly.
  - Add baseline generation, baseline filtering, and CI-aware fail behavior.
  - Finish inline suppressions and adoption guidance so legacy repos can onboard cleanly.
- **Technical Risks**:
  - Unstable fingerprints will cause baseline churn and erode trust. Normalize messages and ranges carefully.
  - Suppressions without discipline can become a dumping ground. Enforce explicit rule IDs and reason strings.
  - Temp-copy module checks can be slow in workspaces. Keep file routing precise and bound work per module.
