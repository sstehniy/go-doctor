# Real-world Validation Record

Date: 2026-02-28

Goal: track runtime, false positives, noisy analyzers to demote/disable, threshold tuning needs, and baseline adoption friction across 5-10 public Go repositories.

## Validation scope

Planned public repositories (8):

1. `github.com/spf13/cobra`
2. `github.com/go-chi/chi`
3. `github.com/sirupsen/logrus`
4. `github.com/hashicorp/raft`
5. `github.com/gofiber/fiber`
6. `github.com/uber-go/zap`
7. `github.com/grpc/grpc-go`
8. `github.com/prometheus/client_golang`

## Current environment status

Network access from the local execution environment is unavailable (`github.com` DNS resolution failed), so public-repo clone runs cannot be executed in this workspace session.

## Recorded local hardening runs

Representative fixture runs completed:

- integration fixtures for API smells, HTTP hardening, SQL leaks, timer leaks, mod hygiene, and clean variants
- workspace discovery fixture (`workspace-multi-module`)
- diff fixture (`diff-mode-changes`)

## Public-run execution command

When network access is available:

```bash
scripts/validate-public-repos.sh
```

The script writes machine-readable outputs into `./artifacts/validation/real-world/<repo>/` and appends summary rows to `./artifacts/validation/real-world/summary.csv`.

## Required fields per repository

- `repo`
- `runtime_seconds`
- `diagnostic_count`
- `error_count`
- `warning_count`
- `noisy_rules`
- `demote_or_disable_candidates`
- `threshold_tuning_notes`
- `baseline_adoption_friction`
