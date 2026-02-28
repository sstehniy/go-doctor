package diagnostics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sstehniy/go-doctor/internal/model"
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

type RetryPolicy struct {
	Attempts           int
	RetryableAnalyzers map[string]struct{}
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		Attempts: 2,
		RetryableAnalyzers: map[string]struct{}{
			"repo-hygiene":  {},
			"govet":         {},
			"staticcheck":   {},
			"govulncheck":   {},
			"golangci-lint": {},
			"custom":        {},
		},
	}
}

func (p RetryPolicy) attempts() int {
	if p.Attempts < 1 {
		return 1
	}
	return p.Attempts
}

func (p RetryPolicy) shouldRetry(name string) bool {
	if len(p.RetryableAnalyzers) == 0 {
		return false
	}
	_, ok := p.RetryableAnalyzers[name]
	return ok
}

func Run(
	ctx context.Context,
	analyzers []Analyzer,
	target Target,
	concurrency int,
	perAnalyzerTimeout time.Duration,
	retryPolicy RetryPolicy,
) ([]model.Diagnostic, []model.ToolError) {
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
				result := runAnalyzerWithPolicy(ctx, analyzer, target, perAnalyzerTimeout, retryPolicy)
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

func runAnalyzerWithPolicy(
	ctx context.Context,
	analyzer Analyzer,
	target Target,
	perAnalyzerTimeout time.Duration,
	retryPolicy RetryPolicy,
) Result {
	attempts := retryPolicy.attempts()
	retryable := retryPolicy.shouldRetry(analyzer.Name())

	for attempt := 1; attempt <= attempts; attempt++ {
		runCtx := ctx
		cancel := func() {}
		if perAnalyzerTimeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, perAnalyzerTimeout)
		}

		resultCh := make(chan Result, 1)
		go func() {
			resultCh <- analyzer.Run(runCtx, target)
		}()

		select {
		case result := <-resultCh:
			cancel()
			return result
		case <-runCtx.Done():
			err := runCtx.Err()
			cancel()

			if err == context.DeadlineExceeded && retryable && attempt < attempts {
				if !waitForRetry(ctx, attempt) {
					return analyzerInfraFailure(analyzer.Name(), ctx.Err().Error())
				}
				continue
			}

			if err == context.DeadlineExceeded {
				return analyzerInfraFailure(analyzer.Name(), timeoutMessage(perAnalyzerTimeout, attempt, attempts))
			}

			return analyzerInfraFailure(analyzer.Name(), err.Error())
		}
	}

	return analyzerInfraFailure(analyzer.Name(), "analyzer execution aborted")
}

func timeoutMessage(timeout time.Duration, attempt int, attempts int) string {
	if timeout <= 0 {
		return fmt.Sprintf("analyzer timed out (attempt %d/%d)", attempt, attempts)
	}
	return fmt.Sprintf("analyzer timed out after %s (attempt %d/%d)", timeout, attempt, attempts)
}

func waitForRetry(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt) * 50 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func analyzerInfraFailure(name string, message string) Result {
	return Result{
		Metadata: AnalyzerMetadata{Name: name},
		ToolErrors: []model.ToolError{
			{
				Tool:    name,
				Message: message,
				Fatal:   true,
			},
		},
	}
}
