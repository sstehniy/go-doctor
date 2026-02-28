package godoctor

import (
	"testing"
	"time"
)

func TestDefaultPerAnalyzerTimeout(t *testing.T) {
	testCases := []struct {
		name  string
		total time.Duration
		want  time.Duration
	}{
		{
			name:  "non-positive timeout disables per-analyzer timeout",
			total: 0,
			want:  0,
		},
		{
			name:  "positive timeout preserves full budget",
			total: 30 * time.Second,
			want:  30 * time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := defaultPerAnalyzerTimeout(testCase.total)
			if got != testCase.want {
				t.Fatalf("defaultPerAnalyzerTimeout(%s) = %s, want %s", testCase.total, got, testCase.want)
			}
		})
	}
}
