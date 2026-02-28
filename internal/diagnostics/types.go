package diagnostics

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type Analyzer interface {
	Name() string
	SupportsDiff() bool
	Run(ctx context.Context, target Target) Result
}

type Target struct {
	RepoRoot         string
	Mode             string
	GoVersion        string
	ModuleRoots      []string
	PackagePatterns  []string
	ModulePatterns   []string
	IncludeFiles     []string
	IncludeGenerated bool
	Architecture     []Layer
}

type Layer struct {
	Name    string
	Include []string
	Allow   []string
}

type AnalyzerMetadata struct {
	Name  string
	Scope string
}

type Result struct {
	Metadata    AnalyzerMetadata
	Diagnostics []model.Diagnostic
	ToolErrors  []model.ToolError
}

func Run(ctx context.Context, analyzers []Analyzer, target Target, concurrency int, perAnalyzerTimeout time.Duration) ([]model.Diagnostic, []model.ToolError) {
	if len(analyzers) == 0 {
		return nil, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}

	type outcome struct {
		result Result
	}

	jobs := make(chan Analyzer)
	results := make(chan outcome, len(analyzers))

	var wg sync.WaitGroup
	workerCount := concurrency
	if workerCount > len(analyzers) {
		workerCount = len(analyzers)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for analyzer := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				runCtx := ctx
				cancel := func() {}
				if perAnalyzerTimeout > 0 {
					runCtx, cancel = context.WithTimeout(ctx, perAnalyzerTimeout)
				}
				result := analyzer.Run(runCtx, target)
				cancel()
				results <- outcome{result: result}
			}
		}()
	}

	go func() {
		for _, analyzer := range analyzers {
			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				close(results)
				return
			case jobs <- analyzer:
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var diagnosticsOut []model.Diagnostic
	var toolErrors []model.ToolError
	for outcome := range results {
		diagnosticsOut = append(diagnosticsOut, outcome.result.Diagnostics...)
		toolErrors = append(toolErrors, outcome.result.ToolErrors...)
	}

	sort.SliceStable(diagnosticsOut, func(i, j int) bool {
		left := diagnosticsOut[i]
		right := diagnosticsOut[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.Plugin != right.Plugin {
			return left.Plugin < right.Plugin
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Message < right.Message
	})

	sort.SliceStable(toolErrors, func(i, j int) bool {
		if toolErrors[i].Tool != toolErrors[j].Tool {
			return toolErrors[i].Tool < toolErrors[j].Tool
		}
		return toolErrors[i].Message < toolErrors[j].Message
	})

	return diagnosticsOut, toolErrors
}
