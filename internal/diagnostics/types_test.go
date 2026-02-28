package diagnostics

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sstehniy/go-doctor/internal/model"
)

type stubAnalyzer struct {
	name   string
	result Result
	runFn  func(context.Context, Target) Result
}

func (s stubAnalyzer) Name() string {
	return s.name
}

func (stubAnalyzer) SupportsDiff() bool {
	return false
}

func (s stubAnalyzer) Run(ctx context.Context, target Target) Result {
	if s.runFn != nil {
		return s.runFn(ctx, target)
	}
	return s.result
}

func TestRunSortsDiagnosticsAndToolErrors(t *testing.T) {
	analyzers := []Analyzer{
		stubAnalyzer{
			name: "b",
			result: Result{
				Diagnostics: []model.Diagnostic{
					{Path: "z.go", Line: 2, Column: 1, Plugin: "b", Rule: "r2", Message: "later"},
				},
				ToolErrors: []model.ToolError{
					{Tool: "ztool", Message: "z err"},
				},
			},
		},
		stubAnalyzer{
			name: "a",
			result: Result{
				Diagnostics: []model.Diagnostic{
					{Path: "a.go", Line: 1, Column: 2, Plugin: "a", Rule: "r1", Message: "first"},
				},
				ToolErrors: []model.ToolError{
					{Tool: "atool", Message: "a err"},
				},
			},
		},
	}

	diagnosticsOut, toolErrors := Run(context.Background(), analyzers, Target{}, 4, 0, RetryPolicy{})
	if len(diagnosticsOut) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diagnosticsOut))
	}
	if diagnosticsOut[0].Path != "a.go" {
		t.Fatalf("expected sorted diagnostics, got %q first", diagnosticsOut[0].Path)
	}
	if len(toolErrors) != 2 {
		t.Fatalf("expected 2 tool errors, got %d", len(toolErrors))
	}
	if toolErrors[0].Tool != "atool" {
		t.Fatalf("expected sorted tool errors, got %q first", toolErrors[0].Tool)
	}
}

func TestRunStopsQueueingAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	analyzers := []Analyzer{
		stubAnalyzer{name: "a"},
		stubAnalyzer{name: "b"},
		stubAnalyzer{name: "c"},
		stubAnalyzer{name: "d"},
		stubAnalyzer{name: "e"},
	}

	done := make(chan struct{})
	go func() {
		Run(ctx, analyzers, Target{}, 2, 0, RetryPolicy{})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run blocked after context cancellation")
	}
}

func TestRunEnforcesPerAnalyzerTimeout(t *testing.T) {
	var calls atomic.Int32
	analyzer := stubAnalyzer{
		name: "slow",
		runFn: func(ctx context.Context, target Target) Result {
			calls.Add(1)
			<-ctx.Done()
			return Result{}
		},
	}

	_, toolErrors := Run(
		context.Background(),
		[]Analyzer{analyzer},
		Target{},
		1,
		20*time.Millisecond,
		RetryPolicy{Attempts: 1, RetryableAnalyzers: map[string]struct{}{"slow": {}}},
	)

	if calls.Load() != 1 {
		t.Fatalf("expected one attempt, got %d", calls.Load())
	}
	if len(toolErrors) != 1 {
		t.Fatalf("expected one timeout tool error, got %#v", toolErrors)
	}
	if !strings.Contains(toolErrors[0].Message, "timed out") {
		t.Fatalf("expected timeout message, got %#v", toolErrors[0])
	}
}

func TestRunRetriesRetryableAnalyzerAfterTimeout(t *testing.T) {
	var calls atomic.Int32
	analyzer := stubAnalyzer{
		name: "flaky",
		runFn: func(ctx context.Context, target Target) Result {
			attempt := calls.Add(1)
			if attempt == 1 {
				<-ctx.Done()
				return Result{}
			}
			return Result{
				Diagnostics: []model.Diagnostic{
					{Path: "main.go", Rule: "custom/rule", Message: "ok"},
				},
			}
		},
	}

	diagnosticsOut, toolErrors := Run(
		context.Background(),
		[]Analyzer{analyzer},
		Target{},
		1,
		20*time.Millisecond,
		RetryPolicy{Attempts: 2, RetryableAnalyzers: map[string]struct{}{"flaky": {}}},
	)

	if calls.Load() != 2 {
		t.Fatalf("expected two attempts, got %d", calls.Load())
	}
	if len(toolErrors) != 0 {
		t.Fatalf("expected retry success with no tool errors, got %#v", toolErrors)
	}
	if len(diagnosticsOut) != 1 {
		t.Fatalf("expected one diagnostic from retry attempt, got %#v", diagnosticsOut)
	}
}
