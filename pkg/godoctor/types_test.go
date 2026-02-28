package godoctor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestDiagnoseDiffNarrowsCustomAnalysisToChangedPackages(t *testing.T) {
	repo := initDiffRepo(t)
	writeRepoFile(t, repo, "pkg/a/a.go", "package a\nfunc Check(err error) bool { return err.Error() == \"boom\" }\n")
	writeRepoFile(t, repo, "pkg/b/b.go", "package b\nfunc Check(err error) bool { return err.Error() == \"boom\" }\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	git(t, repo, "checkout", "-b", "feature")
	writeRepoFile(t, repo, "pkg/a/a.go", "package a\nfunc Check(err error) bool {\n\treturn err.Error() == \"boom\"\n}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "touch pkg a")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		DiffBase:     "main",
		EnableRules:  []string{"error/string-compare"},
		RepoHygiene:  false,
		ThirdParty:   false,
		Custom:       true,
		Score:        true,
		BaselinePath: "",
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one narrowed diagnostic, got %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Path != "pkg/a/a.go" {
		t.Fatalf("expected pkg/a finding, got %#v", result.Diagnostics[0])
	}
}

func TestDiagnoseDiffStillRunsRepoLevelChecks(t *testing.T) {
	repo := initDiffRepo(t)
	writeRepoFile(t, repo, "go.mod", "module example.com/test\n\ngo 1.22.0\n\nreplace example.com/local => ../local\n")
	writeRepoFile(t, repo, "pkg/a/a.go", "package a\nfunc A(){}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	git(t, repo, "checkout", "-b", "feature")
	writeRepoFile(t, repo, "pkg/a/a.go", "package a\nfunc A(){println(\"changed\")}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "touch pkg")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		DiffBase:     "main",
		EnableRules:  []string{"mod/replace-local-path"},
		RepoHygiene:  true,
		ThirdParty:   false,
		Custom:       false,
		Score:        false,
		BaselinePath: "",
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected repo diagnostic, got %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Rule != "mod/replace-local-path" {
		t.Fatalf("expected mod/replace-local-path finding, got %#v", result.Diagnostics[0])
	}
}

func TestDiagnoseDiffSkipsGovulncheckByDefault(t *testing.T) {
	repo := initDiffRepo(t)
	writeRepoFile(t, repo, "main.go", "package main\nfunc main(){}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	git(t, repo, "checkout", "-b", "feature")
	writeRepoFile(t, repo, "main.go", "package main\nfunc main(){println(\"changed\")}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "change")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		DiffBase:     "main",
		EnableRules:  []string{"govulncheck", "mod/not-tidy"},
		RepoHygiene:  true,
		ThirdParty:   true,
		Custom:       false,
		Score:        false,
		BaselinePath: "",
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	foundSkip := false
	for _, skipped := range result.SkippedTools {
		if strings.Contains(skipped, "govulncheck") {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("expected govulncheck skip in diff mode, got %#v", result.SkippedTools)
	}
}

func TestDiagnoseDiffGovulncheckChangedModulesOnlyMode(t *testing.T) {
	repo := initDiffRepo(t)
	writeRepoFile(t, repo, "main.go", "package main\nfunc main(){}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	git(t, repo, "checkout", "-b", "feature")
	writeRepoFile(t, repo, "main.go", "package main\nfunc main(){println(\"changed\")}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "change")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:          30 * time.Second,
		Concurrency:      1,
		DiffBase:         "main",
		DiffGovulncheck:  DiffGovulncheckChangedModulesOnly,
		EnableRules:      []string{"govulncheck", "mod/not-tidy"},
		RepoHygiene:      true,
		ThirdParty:       true,
		Custom:           false,
		Score:            false,
		BaselinePath:     "",
		IncludeGenerated: false,
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	foundSkip := false
	for _, skipped := range result.SkippedTools {
		if strings.Contains(skipped, "govulncheck") {
			foundSkip = true
			break
		}
	}
	if foundSkip {
		t.Fatalf("expected govulncheck to execute in changed-modules-only mode, got skipped tools %#v", result.SkippedTools)
	}
}

func TestDiagnoseDiffDeletedFilesStillRecalculatePackageScope(t *testing.T) {
	repo := initDiffRepo(t)
	writeRepoFile(t, repo, "pkg/a/live.go", "package a\nfunc Check(err error) bool { return err.Error() == \"boom\" }\n")
	writeRepoFile(t, repo, "pkg/a/delete_me.go", "package a\nfunc DeleteMe(){}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	git(t, repo, "checkout", "-b", "feature")
	if err := os.Remove(filepath.Join(repo, "pkg/a/delete_me.go")); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "delete file")

	result, err := Diagnose(context.Background(), repo, Options{
		Timeout:      30 * time.Second,
		Concurrency:  1,
		DiffBase:     "main",
		EnableRules:  []string{"error/string-compare"},
		RepoHygiene:  false,
		ThirdParty:   false,
		Custom:       true,
		Score:        false,
		BaselinePath: "",
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected package to be re-analyzed after delete, got %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Path != "pkg/a/live.go" {
		t.Fatalf("expected finding from live file, got %#v", result.Diagnostics[0])
	}
}

func TestRenderSARIFIncludesRegistryRuleMetadata(t *testing.T) {
	body, err := RenderSARIF(DiagnoseResult{
		Project: ProjectInfo{Root: "/repo"},
		Diagnostics: []Diagnostic{
			{
				Path:     "main.go",
				Line:     1,
				Column:   1,
				Plugin:   "repo",
				Rule:     "fmt/not-gofmt",
				Severity: "info",
				Message:  "file is not gofmt formatted",
			},
		},
	})
	if err != nil {
		t.Fatalf("render sarif: %v", err)
	}

	var payload struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Rules []struct {
						ID   string `json:"id"`
						Help struct {
							Text string `json:"text"`
						} `json:"help"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	if len(payload.Runs) != 1 {
		t.Fatalf("expected one run, got %d", len(payload.Runs))
	}

	helpByRule := map[string]string{}
	for _, rule := range payload.Runs[0].Tool.Driver.Rules {
		helpByRule[rule.ID] = rule.Help.Text
	}
	if helpByRule["repo/fmt/not-gofmt"] == "" {
		t.Fatal("expected repo rule help text from registry metadata")
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

func initDiffRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeRepoFile(t, repo, "go.mod", "module example.com/test\n\ngo 1.22.0\n")
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	return repo
}

func writeRepoFile(t *testing.T, repo string, rel string, content string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output))
}
