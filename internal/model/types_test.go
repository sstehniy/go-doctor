package model

import "testing"

func TestNormalizePath(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unix path",
			input: "./pkg/main.go",
			want:  "pkg/main.go",
		},
		{
			name:  "windows separators",
			input: `pkg\sub\main.go`,
			want:  "pkg/sub/main.go",
		},
		{
			name:  "windows drive path",
			input: `C:\repo\pkg\..\main.go`,
			want:  "C:/repo/main.go",
		},
		{
			name:  "already clean",
			input: "main.go",
			want:  "main.go",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := NormalizePath(testCase.input)
			if got != testCase.want {
				t.Fatalf("NormalizePath(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}
