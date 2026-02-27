# Repository Guidelines

## Structure

This repo is currently planning-first.

- `tasks/`: canonical implementation PRDs in execution order (`m1-...`, `m2-...`)

## Core Commands

Once the Go module is scaffolded:

- `go build ./cmd/go-doctor`
- `go test ./...`
- `go run ./cmd/go-doctor .`

## Testing

Prefer table-driven tests. Keep unit tests near the package and fixture-based integration tests under `testdata/fixtures/`. For analyzers and rules, add both positive and negative cases.

## Contribution Rules

Use short, imperative commit messages, such as `add diff fixtures`. Reference the relevant task doc or milestone in PRs. Do not write code that mutates the checked-out workspace during analysis; use temp copies for repo-level checks.
