# Config Schema

Schema version: `1`.

`go-doctor` auto-discovers config in this order:

1. `.go-doctor.yaml`
2. `.go-doctor.yml`
3. `.go-doctor.json`

CLI flags override config file values.

## Top-level keys

- `scan`
- `output`
- `ci`
- `rules`
- `ignore`
- `thresholds`
- `architecture`
- `analyzers`

## `scan`

- `diff` (`string`): `auto` or explicit base ref. Empty disables diff mode.
- `diffGovulncheck` (`string`): `skip` or `changed-modules-only`.
- `packages` (`[]string`): package scope override.
- `modules` (`[]string`): module scope override.
- `timeout` (`string` duration): global timeout; default `30s`.
- `concurrency` (`int`): worker pool size; default `min(NumCPU, 6)`.
- `baseline` (`string`): path to baseline JSON file.
- `noBaseline` (`bool`): ignore baseline even if configured.
- `includeGenerated` (`bool`): include generated files in analysis.

## `output`

- `format` (`string`): `text`, `json`, `sarif`.
- `path` (`string`): optional output file path.
- `verbose` (`bool`): verbose text output.
- `score` (`bool`): enable/disable score output.
- `quiet` (`bool`): minimal text output.

## `ci`

- `failOn` (`string`): `none`, `info`, `warning`, `error`.

## `rules`

- `enable` (`[]string`): rules or selectors to enable.
- `disable` (`[]string`): rules or selectors to disable.

Selectors include analyzer groups (`repo`, `custom`) and category/prefix groups (for example `context`, `mod`).

## `ignore`

- `rules` (`[]string`): reserved for future static ignore list controls.
- `paths` (`[]string`): reserved for future path ignore controls.

## `thresholds`

- `error` (`int`)
- `warning` (`int`)
- `info` (`int`)

## `architecture`

- `layers` (`[]layer`)
  - `name` (`string`)
  - `include` (`[]string`)
  - `allow` (`[]string`)

## `analyzers`

- `repo` (`bool`)
- `thirdParty` (`bool`)
- `custom` (`bool`)

Defaults:

- `repo=true`
- `thirdParty=true`
- `custom=true`

## Example

```yaml
scan:
  timeout: 45s
  diff: auto
  baseline: .go-doctor-baseline.json
output:
  format: json
ci:
  failOn: warning
rules:
  enable:
    - mod/not-tidy
    - context
analyzers:
  repo: true
  thirdParty: true
  custom: true
```
