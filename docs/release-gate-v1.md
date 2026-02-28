# v1 Release Gate

This checklist mirrors the hardening milestone release gate.

## Formats and core contracts

- [x] Stable `text`, `json`, `sarif` output formats.
- [x] Support for `go.mod` and `go.work` repositories.
- [x] Actionable file/line diagnostics for the majority of findings.
- [x] Support for `--diff`, `--fail-on`, `--list-rules`.
- [x] Baseline and inline suppression support.

## Rule coverage

- [x] At least 18 high-signal custom rules.
- [x] Rule coverage spans at least 8 categories.

## CI and packaging

- [x] GitHub Action SARIF upload flow.
- [x] Cross-platform release packaging workflow (`linux`, `darwin`, `windows`).
- [x] Cross-platform smoke workflow runs packaged binaries.

## Documentation

- [x] Quickstart guide.
- [x] Config schema reference.
- [x] CI usage guide.
- [x] Rule catalog.
- [x] Adoption guide.
