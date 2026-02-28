package custom

import "github.com/stanislavstehniy/go-doctor/internal/diagnostics"

func DefaultAnalyzers(target diagnostics.Target, enableRules []string, disableRules []string) []diagnostics.Analyzer {
	return nil
}
