package custom

import (
	"context"
	"fmt"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type analyzer struct {
	rules []rule
}

func (analyzer) Name() string {
	return "custom"
}

func (analyzer) SupportsDiff() bool {
	return false
}

func (a analyzer) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	result := diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{
			Name:  "custom",
			Scope: "repo",
		},
	}

	pass, toolErrors := loadAnalysisContext(ctx, target)
	result.ToolErrors = append(result.ToolErrors, toolErrors...)
	if pass == nil {
		return result
	}

	for _, rule := range a.rules {
		select {
		case <-ctx.Done():
			result.ToolErrors = append(result.ToolErrors, model.ToolError{
				Tool:    "custom",
				Message: ctx.Err().Error(),
				Fatal:   true,
			})
			return result
		default:
		}
		result.Diagnostics = append(result.Diagnostics, rule.run(pass)...)
	}

	return result
}

func DefaultAnalyzers(_ diagnostics.Target, enableRules []string, disableRules []string) []diagnostics.Analyzer {
	selected, err := selectRules(enableRules, disableRules)
	if err != nil {
		return []diagnostics.Analyzer{failingAnalyzer{err: err}}
	}
	if len(selected) == 0 {
		return nil
	}
	return []diagnostics.Analyzer{analyzer{rules: selected}}
}

type failingAnalyzer struct {
	err error
}

func (f failingAnalyzer) Name() string {
	return "custom"
}

func (failingAnalyzer) SupportsDiff() bool {
	return false
}

func (f failingAnalyzer) Run(ctx context.Context, _ diagnostics.Target) diagnostics.Result {
	return diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{Name: "custom", Scope: "repo"},
		ToolErrors: []model.ToolError{
			{
				Tool:    "custom",
				Message: fmt.Sprintf("resolve custom rules: %v", f.err),
				Fatal:   true,
			},
		},
	}
}
