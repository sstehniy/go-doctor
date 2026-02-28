package godoctor

import (
	"context"
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
