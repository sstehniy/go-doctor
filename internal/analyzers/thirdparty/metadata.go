package thirdparty

import "slices"

type Descriptor struct {
	Plugin      string
	Rule        string
	Severity    string
	Category    string
	Description string
	Help        string
	DocsURL     string
}

func RuleDescriptors() []Descriptor {
	out := []Descriptor{
		{
			Plugin:      "govet",
			Rule:        "govet",
			Severity:    "warning",
			Category:    "correctness",
			Description: "go vet reported a potential issue",
			DocsURL:     "https://pkg.go.dev/cmd/vet",
		},
		{
			Plugin:      "staticcheck",
			Rule:        "staticcheck",
			Severity:    "warning",
			Category:    "maintainability",
			Description: "staticcheck reported a potential issue",
			DocsURL:     "https://staticcheck.dev/docs/checks",
		},
		{
			Plugin:      "govulncheck",
			Rule:        "govulncheck",
			Severity:    "warning",
			Category:    "security",
			Description: "govulncheck reported a vulnerability",
			DocsURL:     "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "golangci-lint",
			Severity:    "warning",
			Category:    "maintainability",
			Description: "golangci-lint reported a linter finding",
			DocsURL:     "https://golangci-lint.run/",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "errcheck",
			Severity:    "warning",
			Category:    "maintainability",
			Description: "Ignored error return value",
			DocsURL:     "https://golangci-lint.run/usage/linters/#errcheck",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "ineffassign",
			Severity:    "warning",
			Category:    "maintainability",
			Description: "Ineffectual assignment",
			DocsURL:     "https://golangci-lint.run/usage/linters/#ineffassign",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "bodyclose",
			Severity:    "warning",
			Category:    "reliability",
			Description: "Response body not closed",
			DocsURL:     "https://golangci-lint.run/usage/linters/#bodyclose",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "rowserrcheck",
			Severity:    "warning",
			Category:    "reliability",
			Description: "rows.Err is not checked",
			DocsURL:     "https://golangci-lint.run/usage/linters/#rowserrcheck",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "sqlclosecheck",
			Severity:    "error",
			Category:    "reliability",
			Description: "sql.Rows.Close is not checked",
			DocsURL:     "https://golangci-lint.run/usage/linters/#sqlclosecheck",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "exportloopref",
			Severity:    "error",
			Category:    "correctness",
			Description: "Loop variable captured by reference",
			DocsURL:     "https://golangci-lint.run/usage/linters/#exportloopref",
		},
		{
			Plugin:      "golangci-lint",
			Rule:        "prealloc",
			Severity:    "warning",
			Category:    "performance",
			Description: "Slice could be preallocated",
			DocsURL:     "https://golangci-lint.run/usage/linters/#prealloc",
		},
		{
			Plugin:      "govet",
			Rule:        "printf",
			Severity:    "error",
			Category:    "correctness",
			Description: "printf-style format mismatch",
			DocsURL:     "https://pkg.go.dev/cmd/vet#hdr-Printf_family",
		},
		{
			Plugin:      "govet",
			Rule:        "copylocks",
			Severity:    "error",
			Category:    "correctness",
			Description: "Lock copied by value",
			DocsURL:     "https://pkg.go.dev/cmd/vet#hdr-Copying_locks",
		},
		{
			Plugin:      "govet",
			Rule:        "lostcancel",
			Severity:    "error",
			Category:    "correctness",
			Description: "Context cancel function is not used",
			DocsURL:     "https://pkg.go.dev/cmd/vet#hdr-CancelFuncs",
		},
		{
			Plugin:      "govet",
			Rule:        "loopclosure",
			Severity:    "error",
			Category:    "correctness",
			Description: "Loop variable captured by closure",
			DocsURL:     "https://pkg.go.dev/cmd/vet#hdr-Loop_variable_captures",
		},
		{
			Plugin:      "govet",
			Rule:        "shift",
			Severity:    "error",
			Category:    "correctness",
			Description: "Invalid shift operation",
			DocsURL:     "https://pkg.go.dev/cmd/vet",
		},
		{
			Plugin:      "govet",
			Rule:        "unmarshal",
			Severity:    "error",
			Category:    "correctness",
			Description: "Suspicious unmarshal target",
			DocsURL:     "https://pkg.go.dev/cmd/vet",
		},
		{
			Plugin:      "govet",
			Rule:        "nilfunc",
			Severity:    "error",
			Category:    "correctness",
			Description: "Nil function comparison",
			DocsURL:     "https://pkg.go.dev/cmd/vet",
		},
		{
			Plugin:      "govet",
			Rule:        "unreachable",
			Severity:    "error",
			Category:    "correctness",
			Description: "Unreachable code",
			DocsURL:     "https://pkg.go.dev/cmd/vet",
		},
	}

	slices.SortFunc(out, func(left, right Descriptor) int {
		leftID := left.Plugin + "/" + left.Rule
		rightID := right.Plugin + "/" + right.Rule
		switch {
		case leftID < rightID:
			return -1
		case leftID > rightID:
			return 1
		default:
			return 0
		}
	})
	return out
}
