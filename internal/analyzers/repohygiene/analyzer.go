package repohygiene

import (
	"context"
	"fmt"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

type analyzer struct {
	rules []rule
}

func (analyzer) Name() string {
	return "repo-hygiene"
}

func (analyzer) SupportsDiff() bool {
	return true
}

func (a analyzer) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	result := diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{
			Name:  "repo-hygiene",
			Scope: "repo",
		},
	}

	pass, toolErrors := loadAnalysisContext(target)
	result.ToolErrors = append(result.ToolErrors, toolErrors...)
	if pass == nil {
		return result
	}

	for _, candidate := range a.rules {
		select {
		case <-ctx.Done():
			result.ToolErrors = append(result.ToolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: ctx.Err().Error(),
				Fatal:   true,
			})
			return result
		default:
		}

		diagnosticsOut, toolErrors := candidate.run(ctx, pass)
		result.Diagnostics = append(result.Diagnostics, diagnosticsOut...)
		result.ToolErrors = append(result.ToolErrors, toolErrors...)
	}

	return result
}

// DefaultAnalyzers returns the default repo hygiene analyzer set.
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

func (failingAnalyzer) Name() string {
	return "repo-hygiene"
}

func (failingAnalyzer) SupportsDiff() bool {
	return true
}

func (f failingAnalyzer) Run(context.Context, diagnostics.Target) diagnostics.Result {
	return diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{Name: "repo-hygiene", Scope: "repo"},
		ToolErrors: []model.ToolError{
			{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("resolve repo hygiene rules: %v", f.err),
				Fatal:   true,
			},
		},
	}
}
