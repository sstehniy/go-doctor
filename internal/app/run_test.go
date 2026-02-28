package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/sstehniy/go-doctor/internal/baseline"
	"github.com/sstehniy/go-doctor/internal/model"
)

func TestRunTextOutputSingleModule(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "single-module")
	configPath := writeNoAnalyzerConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "healthy: no active findings") {
		t.Fatalf("expected healthy output, got %q", output)
	}
	if !strings.Contains(output, "mode: module") {
		t.Fatalf("expected module mode, got %q", output)
	}
	if !strings.Contains(output, "100 / 100") || !strings.Contains(output, "Excellent") {
		t.Fatalf("expected score, got %q", output)
	}
}

func TestVersionUsesLDFlagsValueWhenPresent(t *testing.T) {
	previousVersion := version
	previousReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version = previousVersion
		readBuildInfo = previousReadBuildInfo
	})

	version = "v1.2.3"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, true
	}

	if got := Version(); got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got)
	}
}

func TestVersionUsesBuildInfoWhenDefaultVersionIsDev(t *testing.T) {
	previousVersion := version
	previousReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version = previousVersion
		readBuildInfo = previousReadBuildInfo
	})

	version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.1.2"}}, true
	}

	if got := Version(); got != "0.1.2" {
		t.Fatalf("expected 0.1.2, got %q", got)
	}
}

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	for _, fragment := range []string{
		"Usage:\n  go-doctor [flags] [target]",
		"Examples:",
		"Common Flags",
		"Commands:",
		"completion [shell]",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected help output to contain %q, got %q", fragment, output)
		}
	}
}

func TestRunUnknownFlagReturnsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--wat"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "unknown flag: --wat") {
		t.Fatalf("expected unknown flag error, got %q", errOutput)
	}
	if !strings.Contains(errOutput, usageHint) {
		t.Fatalf("expected usage hint, got %q", errOutput)
	}
}

func TestRunTooManyTargetsReturnsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"first", "second"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "expected at most one target path") {
		t.Fatalf("expected target path error, got %q", errOutput)
	}
	if !strings.Contains(errOutput, usageHint) {
		t.Fatalf("expected usage hint, got %q", errOutput)
	}
}

func TestRunCompletionGuide(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	for _, fragment := range []string{
		"Shell completion install guide.",
		"Bash",
		"Zsh",
		"Fish",
		"PowerShell",
		"go-doctor completion script zsh",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected completion guide to contain %q, got %q", fragment, output)
		}
	}
}

func TestRunCompletionGuideForSingleShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion", "zsh"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Zsh") {
		t.Fatalf("expected zsh guide, got %q", output)
	}
	if !strings.Contains(output, "~/.zsh/completions/_go-doctor") {
		t.Fatalf("expected zsh install path, got %q", output)
	}
	for _, unwanted := range []string{"Bash\n", "Fish\n", "PowerShell\n"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected only zsh section, got %q", output)
		}
	}
}

func TestRunCompletionScripts(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		fragment string
	}{
		{name: "bash", args: []string{"completion", "script", "bash"}, fragment: "__start_go-doctor"},
		{name: "zsh", args: []string{"completion", "script", "zsh"}, fragment: "#compdef go-doctor"},
		{name: "fish", args: []string{"completion", "script", "fish"}, fragment: "complete -c go-doctor"},
		{name: "powershell", args: []string{"completion", "script", "powershell"}, fragment: "Register-ArgumentCompleter"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(context.Background(), testCase.args, &stdout, &stderr)
			if code != ExitSuccess {
				t.Fatalf("expected success, got %d: %s", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got %q", stderr.String())
			}
			if !strings.Contains(stdout.String(), testCase.fragment) {
				t.Fatalf("expected completion output to contain %q, got %q", testCase.fragment, stdout.String())
			}
		})
	}
}

func TestRunCompletionRejectsInvalidShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion", "tcsh"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, `unsupported shell "tcsh"`) {
		t.Fatalf("expected invalid shell error, got %q", errOutput)
	}
	if !strings.Contains(errOutput, usageHint) {
		t.Fatalf("expected usage hint, got %q", errOutput)
	}
}

func TestRunCompletionScriptRejectsMissingShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion", "script"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "accepts 1 arg(s), received 0") {
		t.Fatalf("expected missing arg error, got %q", errOutput)
	}
	if !strings.Contains(errOutput, usageHint) {
		t.Fatalf("expected usage hint, got %q", errOutput)
	}
}

func TestRunJSONOutputWorkspace(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "workspace")
	configPath := writeNoAnalyzerConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, "--format=json", fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}

	var payload struct {
		SchemaVersion int `json:"schemaVersion"`
		Project       struct {
			Mode         string   `json:"mode"`
			PackageCount int      `json:"packageCount"`
			ModuleRoots  []string `json:"moduleRoots"`
		} `json:"project"`
		Diagnostics []any `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", payload.SchemaVersion)
	}
	if payload.Project.Mode != "workspace" {
		t.Fatalf("expected workspace mode, got %q", payload.Project.Mode)
	}
	if payload.Project.PackageCount != 2 {
		t.Fatalf("expected 2 packages, got %d", payload.Project.PackageCount)
	}
	if len(payload.Project.ModuleRoots) != 2 {
		t.Fatalf("expected 2 module roots, got %d", len(payload.Project.ModuleRoots))
	}
	if len(payload.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(payload.Diagnostics))
	}
}

func TestRunSARIFOutputWorkspace(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "workspace")
	configPath := writeNoAnalyzerConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, "--format=sarif", fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload struct {
		Version string `json:"version"`
		Runs    []struct {
			Results []any `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	if payload.Version != "2.1.0" {
		t.Fatalf("expected SARIF version 2.1.0, got %q", payload.Version)
	}
	if len(payload.Runs) != 1 {
		t.Fatalf("expected one run, got %d", len(payload.Runs))
	}
	if len(payload.Runs[0].Results) != 0 {
		t.Fatalf("expected no findings, got %d", len(payload.Runs[0].Results))
	}
}

func TestRunJSONOutputRepoHygiene(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "repo-hygiene", "not-tidy")
	configPath := writeRepoOnlyConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, "--format=json", "--enable=mod/not-tidy", fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload struct {
		Diagnostics []struct {
			Plugin string `json:"plugin"`
			Rule   string `json:"rule"`
			Path   string `json:"path"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(payload.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d", len(payload.Diagnostics))
	}
	if payload.Diagnostics[0].Plugin != "repo" || payload.Diagnostics[0].Rule != "mod/not-tidy" || payload.Diagnostics[0].Path != "go.mod" {
		t.Fatalf("unexpected diagnostic: %#v", payload.Diagnostics[0])
	}
}

func TestRunGeneratesBaselineAndSuppressesCurrentFindings(t *testing.T) {
	t.Setenv("CI", "false")

	repo := writeRepoHygieneFixture(t)
	configPath := writeRepoOnlyConfig(t)
	baselinePath := filepath.Join(t.TempDir(), "artifacts", "baseline.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(
		context.Background(),
		[]string{"--config", configPath, "--format=json", "--baseline", baselinePath, "--fail-on=info", "--enable=fmt/not-gofmt,license/missing", repo},
		&stdout,
		&stderr,
	)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload struct {
		Diagnostics []struct {
			Rule       string `json:"rule"`
			Suppressed bool   `json:"suppressed"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(payload.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(payload.Diagnostics))
	}
	for _, diagnostic := range payload.Diagnostics {
		if !diagnostic.Suppressed {
			t.Fatalf("expected generated baseline to suppress %s", diagnostic.Rule)
		}
	}

	file, _, err := baseline.Load(baselinePath)
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 baseline entries, got %d", len(file.Entries))
	}
}

func TestRunCIBaselineFailsOnlyOnNewFindings(t *testing.T) {
	t.Setenv("CI", "true")

	repo := writeRepoHygieneFixture(t)
	configPath := writeRepoOnlyConfig(t)
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := baseline.Write(baselinePath, []model.Diagnostic{
		{
			Rule:      "license/missing",
			Path:      "go.mod",
			Line:      1,
			Column:    1,
			EndLine:   1,
			EndColumn: 1,
			Message:   "repository is missing a license file",
		},
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		context.Background(),
		[]string{"--config", configPath, "--format=json", "--baseline", baselinePath, "--fail-on=info", "--enable=fmt/not-gofmt,license/missing", repo},
		&stdout,
		&stderr,
	)
	if code != ExitFailure {
		t.Fatalf("expected failure for new finding, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload struct {
		Diagnostics []struct {
			Rule       string `json:"rule"`
			Suppressed bool   `json:"suppressed"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(payload.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(payload.Diagnostics))
	}

	var suppressed int
	for _, diagnostic := range payload.Diagnostics {
		if diagnostic.Suppressed {
			suppressed++
		}
	}
	if suppressed != 1 {
		t.Fatalf("expected exactly one suppressed finding, got %d", suppressed)
	}
}

func TestRunCIMissingBaselineFailsFatal(t *testing.T) {
	t.Setenv("CI", "true")

	repo := writeRepoHygieneFixture(t)
	configPath := writeRepoOnlyConfig(t)
	baselinePath := filepath.Join(t.TempDir(), "missing-baseline.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(
		context.Background(),
		[]string{"--config", configPath, "--baseline", baselinePath, "--enable=license/missing", repo},
		&stdout,
		&stderr,
	)
	if code != ExitFatal {
		t.Fatalf("expected fatal exit, got %d", code)
	}
	if !strings.Contains(stderr.String(), "does not exist in CI") {
		t.Fatalf("expected CI baseline error, got %q", stderr.String())
	}
}

func TestRunNoBaselineDisablesExistingBaseline(t *testing.T) {
	t.Setenv("CI", "true")

	repo := writeRepoHygieneFixture(t)
	configPath := writeRepoOnlyConfig(t)
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := baseline.Write(baselinePath, []model.Diagnostic{
		{
			Rule:      "license/missing",
			Path:      "go.mod",
			Line:      1,
			Column:    1,
			EndLine:   1,
			EndColumn: 1,
			Message:   "repository is missing a license file",
		},
		{
			Rule:      "fmt/not-gofmt",
			Path:      "main.go",
			Line:      1,
			Column:    1,
			EndLine:   1,
			EndColumn: 1,
			Message:   "file is not gofmt formatted",
		},
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		context.Background(),
		[]string{"--config", configPath, "--format=json", "--baseline", baselinePath, "--no-baseline", "--fail-on=info", "--enable=fmt/not-gofmt,license/missing", repo},
		&stdout,
		&stderr,
	)
	if code != ExitFailure {
		t.Fatalf("expected failure when baseline is disabled, got %d: %s", code, stderr.String())
	}

	var payload struct {
		Diagnostics []struct {
			Suppressed bool `json:"suppressed"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	for _, diagnostic := range payload.Diagnostics {
		if diagnostic.Suppressed {
			t.Fatal("expected no diagnostics to be suppressed when --no-baseline is set")
		}
	}
}

func writeNoAnalyzerConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".go-doctor.yaml")
	if err := os.WriteFile(path, []byte("analyzers:\n  repo: false\n  thirdParty: false\n  custom: false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeRepoOnlyConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".go-doctor.yaml")
	if err := os.WriteFile(path, []byte("analyzers:\n  repo: true\n  thirdParty: false\n  custom: false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeRepoHygieneFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/repohygiene\n\ngo 1.22.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){println(\"hi\")}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return root
}

func TestNormalizeArgs(t *testing.T) {
	testCases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "bare diff uses auto mode",
			in:   []string{"--diff"},
			want: []string{"--diff=auto"},
		},
		{
			name: "bare diff keeps trailing target positional",
			in:   []string{"--diff", "."},
			want: []string{"--diff=auto", "."},
		},
		{
			name: "explicit base remains explicit",
			in:   []string{"--diff", "origin/main", "."},
			want: []string{"--diff", "origin/main", "."},
		},
		{
			name: "explicit base without target remains explicit",
			in:   []string{"--diff", "origin/main"},
			want: []string{"--diff", "origin/main"},
		},
		{
			name: "explicit base main remains explicit",
			in:   []string{"--diff", "main"},
			want: []string{"--diff", "main"},
		},
		{
			name: "bare diff before another flag uses auto mode",
			in:   []string{"--diff", "--format=json"},
			want: []string{"--diff=auto", "--format=json"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := normalizeArgs(testCase.in)
			if len(got) != len(testCase.want) {
				t.Fatalf("normalizeArgs(%#v) len = %d, want %d (%#v)", testCase.in, len(got), len(testCase.want), got)
			}
			for index := range got {
				if got[index] != testCase.want[index] {
					t.Fatalf("normalizeArgs(%#v)[%d] = %q, want %q (%#v)", testCase.in, index, got[index], testCase.want[index], got)
				}
			}
		})
	}
}
