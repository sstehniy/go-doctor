# go-doctor GitHub Action

This action:

1. installs a prebuilt `go-doctor` release binary
2. runs `go-doctor --format sarif --output results.sarif`
3. uploads SARIF with `github/codeql-action/upload-sarif` by default
4. exposes `score`, `findings`, `errors`, and `warnings` outputs

## Inputs

- `directory` (default: `.`)
- `format` (default: `sarif`, optional second pass supports `json`)
- `fail-on` (default: `error`)
- `diff` (default: empty)
- `config` (default: empty)
- `github-token` (default: empty, optional for release API auth)
- `version` (default: `latest`)
- `repository` (default: `stanislavstehniy/go-doctor`)
- `upload-sarif` (default: `true`)

## Outputs

- `score`
- `findings`
- `errors`
- `warnings`
- `sarif-path`

## Example

```yaml
permissions:
  contents: read
  security-events: write

steps:
  - uses: actions/checkout@v4
  - uses: ./.github/actions/go-doctor
    with:
      directory: .
      fail-on: warning
```

For a full end-to-end workflow example with an explicit `upload-sarif` step, see:
`docs/examples/go-doctor-sarif-workflow.yml`.

