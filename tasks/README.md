# Task Index

This directory contains the full implementation PRDs for `go-doctor`, ordered by recommended execution sequence. Together, these files are the complete execution plan.

## Execution Order

1. [Milestone 1: Core Skeleton](./m1-core-skeleton.md) (Done)
   Establish the CLI foundation, config loading, project discovery, stable result types, and text/JSON output.
2. [Milestone 2: Third-Party Adapters](./m2-third-party-adapters.md)
   Integrate `go vet`, `staticcheck`, `govulncheck`, and the approved `golangci` subset through a shared analyzer interface.
3. [Milestone 3: Custom Rules v1](./m3-custom-rules-v1.md)
   Add first-party AST and type-aware rules, plus the rule registry and config-driven enable/disable support.
4. [Milestone 4: Repo Hygiene + Adoption Controls](./m4-repo-hygiene-adoption-controls.md)
   Add repo-level analyzers, baselines, inline suppressions, and onboarding controls for legacy repos.
5. [Milestone 5: Scoring + Diff](./m5-scoring-diff.md)
   Implement health scoring, Git-based diff narrowing, and skipped-tool reporting.
6. [Milestone 6: SARIF + GitHub Action](./m6-sarif-github-action.md)
   Export SARIF and package `go-doctor` for GitHub code scanning workflows.
7. [Milestone 7: Hardening](./m7-hardening.md)
   Finish performance tuning, timeout policy, release packaging, smoke tests, and release-grade documentation.

## Notes

- The original source plan used a `Milestone 3.5` label for repo hygiene and adoption controls.
- In this task set, that work is normalized to `Milestone 4` so filenames stay sequential and easy to scan.
