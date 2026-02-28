package scoring

import (
	"testing"

	"github.com/sstehniy/go-doctor/internal/model"
)

func TestScoreMathAndMultipliers(t *testing.T) {
	diagnosticsOut := []model.Diagnostic{
		{Plugin: "custom", Rule: "error/a", Severity: "error", Category: "correctness"},
		{Plugin: "custom", Rule: "error/a", Severity: "error", Category: "correctness"},
		{Plugin: "custom", Rule: "error/a", Severity: "error", Category: "correctness"},
		{Plugin: "staticcheck", Rule: "SA1000", Severity: "warning", Category: "maintainability"},
	}

	score := Score(diagnosticsOut, true)
	// correctness errors (3): (8 + 2*1) * 1.25 = 12.5
	// style-like warning: 3 * 0.5 = 1.5
	// total penalty = 14.0 -> score 86
	if score.Value != 86 {
		t.Fatalf("expected score 86, got %d", score.Value)
	}
	if score.Grade != "Good" {
		t.Fatalf("expected Good grade, got %q", score.Grade)
	}
}

func TestScoreOccurrenceCaps(t *testing.T) {
	var diagnosticsOut []model.Diagnostic
	for i := 0; i < 20; i++ {
		diagnosticsOut = append(diagnosticsOut, model.Diagnostic{
			Plugin:   "custom",
			Rule:     "concurrency/waitgroup-misuse",
			Severity: "error",
			Category: "concurrency",
		})
	}
	for i := 0; i < 20; i++ {
		diagnosticsOut = append(diagnosticsOut, model.Diagnostic{
			Plugin:   "custom",
			Rule:     "perf/defer-in-hot-loop",
			Severity: "warning",
			Category: "performance",
		})
	}

	score := Score(diagnosticsOut, true)
	// error cap: (8 + 5) * 1.25 = 16.25
	// warning cap: (3 + 3) * 0.75 = 4.5
	// total = 20.75 => 79
	if score.Value != 79 {
		t.Fatalf("expected score 79, got %d", score.Value)
	}
}

func TestScoreLabels(t *testing.T) {
	testCases := []struct {
		name      string
		in        []model.Diagnostic
		wantScore int
		wantGrade string
	}{
		{
			name: "excellent lower bound 90",
			in: []model.Diagnostic{
				{Plugin: "custom", Rule: "error/a", Severity: "error", Category: "correctness"}, // 10 points
			},
			wantScore: 90,
			wantGrade: "Excellent",
		},
		{
			name: "good lower bound 75",
			in: append(
				diagnosticsOf("error", "mod", 2),
				diagnosticsOf("warning", "mod", 3)...,
			), // 25 points
			wantScore: 75,
			wantGrade: "Good",
		},
		{
			name: "needs work lower bound 50",
			in: append(
				diagnosticsOf("error", "mod", 4),
				diagnosticsOf("warning", "mod", 6)...,
			), // 50 points
			wantScore: 50,
			wantGrade: "Needs work",
		},
		{
			name: "critical upper bound 49",
			in: append(
				diagnosticsOf("error", "mod", 6),
				diagnosticsOf("warning", "mod", 1)...,
			), // 51 points
			wantScore: 49,
			wantGrade: "Critical",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := Score(testCase.in, true)
			if result.Value != testCase.wantScore {
				t.Fatalf("expected score %d, got %d", testCase.wantScore, result.Value)
			}
			if result.Grade != testCase.wantGrade {
				t.Fatalf("expected grade %q, got %q", testCase.wantGrade, result.Grade)
			}
		})
	}
}

func TestScoreFloors(t *testing.T) {
	t.Run("reachable govulncheck caps score to critical range", func(t *testing.T) {
		result := Score([]model.Diagnostic{
			{Plugin: "govulncheck", Rule: "GO-2024-0001", Severity: "error", Category: "security"},
		}, true)
		if result.Value != 49 {
			t.Fatalf("expected reachable govulncheck to cap score at 49, got %d", result.Value)
		}
		if result.Grade != "Critical" {
			t.Fatalf("expected Critical grade, got %q", result.Grade)
		}
	})

	t.Run("mod readonly failure caps score to needs work range", func(t *testing.T) {
		result := Score([]model.Diagnostic{
			{Plugin: "repo", Rule: "build/mod-readonly-failure", Severity: "error", Category: "build"},
		}, true)
		if result.Value != 74 {
			t.Fatalf("expected mod readonly failure to cap score at 74, got %d", result.Value)
		}
		if result.Grade != "Needs work" {
			t.Fatalf("expected Needs work grade, got %q", result.Grade)
		}
	})
}

func TestScoreIgnoresSuppressedDiagnosticsAndIsDeterministic(t *testing.T) {
	diagnosticsOut := []model.Diagnostic{
		{Plugin: "custom", Rule: "rule/a", Severity: "error", Category: "correctness"},
		{Plugin: "custom", Rule: "rule/a", Severity: "error", Category: "correctness", Suppressed: true},
		{Plugin: "custom", Rule: "rule/b", Severity: "warning", Category: "testing"},
	}

	want := Score(diagnosticsOut, true)
	for i := 0; i < 100; i++ {
		got := Score(diagnosticsOut, true)
		if got != want {
			t.Fatalf("score changed between runs: want %#v got %#v", want, got)
		}
	}
}

func TestScoreDisabled(t *testing.T) {
	result := Score([]model.Diagnostic{{Severity: "error"}}, false)
	if result.Enabled {
		t.Fatalf("expected disabled score, got %#v", result)
	}
}

func diagnosticsOf(severity string, category string, count int) []model.Diagnostic {
	out := make([]model.Diagnostic, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, model.Diagnostic{
			Plugin:   "custom",
			Rule:     severity + "/r" + string(rune('a'+i)),
			Severity: severity,
			Category: category,
		})
	}
	return out
}
