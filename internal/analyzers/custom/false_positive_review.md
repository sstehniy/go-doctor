# Custom Rule False-Positive Review

Date: 2026-02-28

This milestone requires a public-repo false-positive review before default-on custom rules are treated as release-ready. The review was run against the current implementation with only custom analyzers enabled and default rule settings.

Repos reviewed:

- `github.com/go-chi/chi`
- `github.com/spf13/cobra`
- `github.com/sirupsen/logrus`

Execution notes:

- Each repo was cloned shallowly into `/tmp/go-doctor-m3-review`.
- `go-doctor` was built from the current workspace.
- Runs used a temporary config with `analyzers.thirdParty=false`, `analyzers.custom=true`, `output.format=json`, and `ci.failOn=none`.

Results:

- `go-chi/chi`: 104 diagnostics.
  Dominant rules: `net/http-request-without-context` (20), `error/ignored-return` (14), `context/missing-first-arg` (13), `perf/fmt-sprint-simple-concat` (12), `net/http-server-no-timeouts` (9).
- `spf13/cobra`: 188 diagnostics.
  Dominant rules: `error/ignored-return` (78), `test/no-assertions` (66), `perf/fmt-sprint-simple-concat` (25), `api/exported-mutable-global` (6), `arch/god-file` (5).
- `sirupsen/logrus`: 117 diagnostics.
  Dominant rules: `test/no-assertions` (59), `concurrency/mutex-copy` (22), `error/ignored-return` (21), `arch/god-file` (4), `test/sleep-in-test` (3).

Current assessment:

- The review was completed on three public repos as required.
- Several default-on heuristics remain noisy in real codebases, especially `test/no-assertions`, `error/ignored-return`, `context/missing-first-arg`, `net/http-request-without-context`, and `perf/fmt-sprint-simple-concat`.
- Based on this review, the following rules were moved behind opt-in config:
  `test/no-assertions`, `error/ignored-return`, `context/missing-first-arg`, `net/http-request-without-context`, and `perf/fmt-sprint-simple-concat`.
- The implementation is complete for Milestone 3 with noisy rules moved behind opt-in config as required by the milestone guidance.
