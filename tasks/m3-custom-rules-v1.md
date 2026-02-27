# 1. Executive Summary

- **Problem Statement**: Third-party analyzers cover important ground, but they do not capture the outage-preventing, product-specific heuristics that make `go-doctor` distinct. The third milestone must add first-party rules that target real Go reliability issues with low false positives.
- **Proposed Solution**: Implement the first batch of custom AST and type-aware rules across the planned categories, back them with a rule registry and documentation, and make rule enablement configurable.
- **Success Criteria**:
  - At least the planned first-pass rules across error handling, context, concurrency, resource/net reliability, security, API surface, library safety, architecture, and performance are implemented behind a common registry.
  - Every shipped rule has one positive fixture and one negative fixture.
  - Each rule emits remediation help text and maps into the shared `Diagnostic` model.
  - Config-based enable/disable works for individual rules and groups.
  - False-positive review is completed on at least three public sample repos before a rule remains default-on.

# 2. User Experience & Functionality

- **User Personas**:
  - Solo maintainer authoring high-signal Go rules.
  - Go engineer who wants findings that prevent runtime issues, not stylistic churn.
  - CI adopter who needs predictable, explainable custom findings.
- **User Stories**:
  - As a maintainer, I want a reusable rule framework so new first-party checks can be added without bespoke plumbing each time.
  - As a Go engineer, I want findings that point to a concrete file and line with a clear fix path.
  - As a CI adopter, I want low-noise defaults so I can enable custom rules in CI without drowning in false positives.
- **Acceptance Criteria**:
  - Add custom analyzer packages under `internal/analyzers/custom` using `go/ast`, `go/token`, `go/types`, and `go/packages`.
  - Implement the full initial rule inventory:
    `error/ignored-return`, `error/string-compare`, `error/fmt-error-without-wrap`,
    `context/missing-first-arg`, `context/background-in-request-path`, `context/not-propagated`, `context/with-timeout-not-canceled`,
    `concurrency/go-routine-leak-risk`, `concurrency/ticker-not-stopped`, `concurrency/waitgroup-misuse`, `concurrency/mutex-copy`,
    `perf/defer-in-hot-loop`, `perf/fmt-sprint-simple-concat`, `perf/bytes-buffer-copy`, `perf/json-unmarshal-twice`,
    `net/http-request-without-context`, `net/http-default-client`, `net/http-server-no-timeouts`,
    `time/tick-leak`, `time/after-in-loop`,
    `db/rows-not-closed`, `db/rows-err-not-checked`, `db/tx-no-deferred-rollback`,
    `io/readall-unbounded` (default OFF),
    `arch/cross-layer-import`, `arch/forbidden-package-cycles`, `arch/oversized-package`, `arch/god-file`,
    `test/missing-table-driven` (default OFF), `test/no-assertions`, `test/sleep-in-test`, `test/http-handler-no-test`,
    `sec/math-rand-for-secret`, `sec/insecure-temp-file`, `sec/exec-user-input`,
    `api/error-string-branching`, `api/exported-mutable-global`, `api/init-side-effects`,
    `lib/os-exit-in-non-main`, and `lib/flag-parse-in-non-main`.
  - `Resource` is treated as a first-class category in diagnostics and future scoring.
  - Add a rule registry with stable descriptors: plugin, rule, category, default-on state, severity, description, and help text.
  - `--enable` and `--disable` resolve rules and groups deterministically; unknown rule names fail fast.
  - Generated files remain excluded by default, with config control for overrides.
- **Non-Goals**:
  - Whole-program taint analysis.
  - Autofix or automated rewrites.
  - Shipping subjective style-only rules as default-on behavior.

# 3. AI System Requirements (If Applicable)

- **Tool Requirements**: Not applicable. This milestone uses deterministic AST and type analysis only.
- **Evaluation Strategy**: Use per-rule fixture coverage, line/column assertions, and real-world false-positive trials. Rules exceeding the false-positive threshold move behind opt-in config.

# 4. Technical Specifications

- **Architecture Overview**:
  - Build a shared custom rule execution layer that loads packages once per scope, then runs multiple rule visitors against the same syntax and type info where practical.
  - Represent custom rules through the same analyzer interface used by third-party adapters so output, scoring, diff mode, and suppressions can treat them uniformly.
  - Centralize rule metadata in a registry that also feeds `ListRules()` and future documentation generation.
- **Integration Points**:
  - Config system from Milestone 1 for rule enable/disable and thresholds.
  - Diagnostic model for normalized results, remediation help text, and categories.
  - Fixture suite under `testdata/fixtures` for both positive and negative coverage.
  - `arch/cross-layer-import` defaults to convention rules where `internal/domain` cannot import `internal/transport` and `internal/platform` cannot import `internal/transport` unless overridden by config-defined layers.
- **Security & Privacy**:
  - Analyze source statically only; do not execute repository code.
  - Exclude generated and vendor code by default to reduce noise and unexpected scan scope.
  - Keep heuristics conservative where user input or taint is ambiguous.
  - Default-on rules must remain objective, low-noise, and useful without whole-program taint analysis; rules above the false-positive threshold move behind opt-in config.

# 5. Risks & Roadmap

- **Implementation Sequence**:
  - Build the rule registry and shared package-loading layer.
  - Implement the full initial rule set and wire config-based enable/disable behavior.
  - Complete fixture coverage and real-world false-positive review before the rules are treated as release-ready.
- **Technical Risks**:
  - Over-eager AST heuristics can create noisy findings. Start with objective patterns and require fixture plus public-repo validation.
  - Re-loading packages per rule will hurt performance. Share package/type information across rules where possible.
  - If rule IDs change later, baselines and suppressions will become fragile. Freeze naming conventions early.
