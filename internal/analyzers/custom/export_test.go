package custom

import "github.com/sstehniy/go-doctor/internal/diagnostics"

func ExportDefaultAnalyzers(target diagnostics.Target, enableRules []string, disableRules []string) []diagnostics.Analyzer {
	return DefaultAnalyzers(target, enableRules, disableRules)
}
