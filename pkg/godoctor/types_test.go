package godoctor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDiagnoseDoesNotCreateBaselineWhenToolErrorsExist(t *testing.T) {
	repo := writeRepoFixture(t, "package main\nfunc main(){println(\"hi\")}\n")
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	toolDir := toolPathWithGofmtOnly(t)
	t.Setenv("PATH", toolDir)

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		EnableRules:  []string{"fmt/not-gofmt", "staticcheck"},
		BaselinePath: baselinePath,
		RepoHygiene:  true,
		ThirdParty:   true,
		Custom:       false,
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.ToolErrors) == 0 {
		t.Fatal("expected tool errors")
	}
	if _, err := os.Stat(baselinePath); !os.IsNotExist(err) {
		t.Fatalf("expected no baseline file to be created, got err=%v", err)
	}
}

func TestDiagnoseDoesNotCreateBaselineWhenAllAnalyzersFail(t *testing.T) {
	repo := writeRepoFixture(t, "package main\nfunc main() {}\n")
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	t.Setenv("PATH", t.TempDir())

	_, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		EnableRules:  []string{"staticcheck"},
		BaselinePath: baselinePath,
		RepoHygiene:  false,
		ThirdParty:   true,
		Custom:       false,
	})
	if err == nil {
		t.Fatal("expected diagnose to fail when all analyzers fail")
	}
	if _, statErr := os.Stat(baselinePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no baseline file to be created, got err=%v", statErr)
	}
}

func TestDiagnoseAppliesInlineSuppressions(t *testing.T) {
	repo := writeRepoFixture(t, "package main\nfunc check(err error) bool {\n\treturn err.Error() == \"boom\" // godoctor:ignore error/string-compare legacy sentinel mismatch\n}\nfunc main() {}\n")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:     30 * time.Second,
		Concurrency: 1,
		EnableRules: []string{"error/string-compare"},
		RepoHygiene: false,
		ThirdParty:  false,
		Custom:      true,
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.ToolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", result.ToolErrors)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", result.Diagnostics)
	}
	if !result.Diagnostics[0].Suppressed {
		t.Fatalf("expected diagnostic to be suppressed, got %#v", result.Diagnostics[0])
	}
}

func TestDiagnoseReportsInvalidInlineSuppressions(t *testing.T) {
	repo := writeRepoFixture(t, "package main\nfunc check(err error) bool {\n\treturn err.Error() == \"boom\" // godoctor:ignore error/string-compare\n}\nfunc main() {}\n")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:     30 * time.Second,
		Concurrency: 1,
		EnableRules: []string{"error/string-compare"},
		RepoHygiene: false,
		ThirdParty:  false,
		Custom:      true,
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected two diagnostics, got %#v", result.Diagnostics)
	}

	byRule := map[string]Diagnostic{}
	for _, diagnostic := range result.Diagnostics {
		byRule[diagnostic.Rule] = diagnostic
	}
	if byRule["error/string-compare"].Suppressed {
		t.Fatalf("expected malformed suppression to leave original finding active, got %#v", byRule["error/string-compare"])
	}
	if byRule["suppress/invalid"].Rule != "suppress/invalid" {
		t.Fatalf("expected invalid suppression diagnostic, got %#v", result.Diagnostics)
	}
}

func TestDiagnoseBaselineSkipsInlineSuppressedFindings(t *testing.T) {
	repo := writeRepoFixture(t, "package main\nfunc check(err error) bool {\n\treturn err.Error() == \"boom\" // godoctor:ignore error/string-compare legacy sentinel mismatch\n}\nfunc main() {}\n")
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		EnableRules:  []string{"error/string-compare"},
		BaselinePath: baselinePath,
		RepoHygiene:  false,
		ThirdParty:   false,
		Custom:       true,
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.Diagnostics) != 1 || !result.Diagnostics[0].Suppressed {
		t.Fatalf("expected one suppressed diagnostic, got %#v", result.Diagnostics)
	}

	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var file struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("unmarshal baseline: %v", err)
	}
	if len(file.Entries) != 0 {
		t.Fatalf("expected inline-suppressed findings to stay out of the baseline, got %d entries", len(file.Entries))
	}
}

func writeRepoFixture(t *testing.T, mainGo string) string {
	t.Helper()

	root := t.TempDir()
	writeFile := func(name string, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	writeFile("go.mod", "module example.com/test\n\ngo 1.22.0\n")
	writeFile("main.go", mainGo)
	return root
}

func toolPathWithGofmtOnly(t *testing.T) string {
	t.Helper()

	gofmtPath, err := exec.LookPath("gofmt")
	if err != nil {
		t.Fatalf("look up gofmt: %v", err)
	}

	dir := t.TempDir()
	linkPath := filepath.Join(dir, "gofmt")
	if err := os.Symlink(gofmtPath, linkPath); err != nil {
		t.Fatalf("symlink gofmt: %v", err)
	}
	return dir
}
