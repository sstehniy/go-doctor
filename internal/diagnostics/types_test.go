package diagnostics

import (
	"context"
	"testing"
	"time"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type stubAnalyzer struct {
	name   string
	result Result
}

func (s stubAnalyzer) Name() string {
	return s.name
}

func (stubAnalyzer) SupportsDiff() bool {
	return false
}

func (s stubAnalyzer) Run(ctx context.Context, target Target) Result {
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

	diagnosticsOut, toolErrors := Run(context.Background(), analyzers, Target{}, 4, 0)
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
		Run(ctx, analyzers, Target{}, 2, 0)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run blocked after context cancellation")
	}
}
