package text

import (
	"strings"
	"testing"

	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
)

func TestRenderQuietHealthyReturnsNil(t *testing.T) {
	result := godoctor.DiagnoseResult{}
	body := Render(result, Options{Quiet: true})
	if body != nil {
		t.Fatalf("expected nil body for quiet healthy output, got %q", string(body))
	}
}

func TestRenderGroupsDiagnosticsAndTruncatesCompactFiles(t *testing.T) {
	result := godoctor.DiagnoseResult{
		Project: godoctor.ProjectInfo{Root: "/repo", Mode: "module", GoVersion: "1.23.0", ModuleRoots: []string{"/repo"}, PackageCount: 3},
		Diagnostics: []godoctor.Diagnostic{
			{Plugin: "custom", Rule: "arch/god-file", Severity: "warning", Category: "architecture", Message: "file exceeds threshold", Help: "split large files", Path: "a.go", Line: 10},
			{Plugin: "custom", Rule: "arch/god-file", Severity: "warning", Category: "architecture", Message: "file exceeds threshold", Help: "split large files", Path: "b.go", Line: 20},
			{Plugin: "custom", Rule: "arch/god-file", Severity: "warning", Category: "architecture", Message: "file exceeds threshold", Help: "split large files", Path: "c.go", Line: 30},
		},
		Score: &godoctor.ScoreResult{Enabled: true, Value: 77, Max: 100, Grade: "Good"},
	}

	body := string(Render(result, Options{UseUnicode: false, MaxCompactFiles: 2}))
	if !strings.Contains(body, "W custom/arch/god-file (3)") {
		t.Fatalf("expected grouped warning header, got %q", body)
	}
	if !strings.Contains(body, "+1 more files") {
		t.Fatalf("expected truncated compact files, got %q", body)
	}
	if strings.Contains(body, "Suppressed Findings") {
		t.Fatalf("did not expect suppressed section, got %q", body)
	}
}

func TestRenderVerboseShowsAllLocations(t *testing.T) {
	result := godoctor.DiagnoseResult{
		Project: godoctor.ProjectInfo{Root: "/repo", Mode: "module", GoVersion: "1.23.0", ModuleRoots: []string{"/repo"}, PackageCount: 1},
		Diagnostics: []godoctor.Diagnostic{
			{Plugin: "custom", Rule: "error/string-compare", Severity: "warning", Category: "correctness", Message: "string compare", Path: "pkg/a.go", Line: 7, Column: 2},
			{Plugin: "custom", Rule: "error/string-compare", Severity: "warning", Category: "correctness", Message: "string compare", Path: "pkg/b.go", Line: 9, Column: 1},
		},
	}

	body := string(Render(result, Options{Verbose: true, UseUnicode: false}))
	if !strings.Contains(body, "pkg/a.go:7:2") || !strings.Contains(body, "pkg/b.go:9:1") {
		t.Fatalf("expected verbose locations, got %q", body)
	}
}

func TestRenderSeparatesSuppressedFindings(t *testing.T) {
	result := godoctor.DiagnoseResult{
		Project: godoctor.ProjectInfo{Root: "/repo", Mode: "module", GoVersion: "1.23.0", ModuleRoots: []string{"/repo"}, PackageCount: 1},
		Diagnostics: []godoctor.Diagnostic{
			{Plugin: "repo", Rule: "mod/not-tidy", Severity: "warning", Category: "mod", Message: "go.mod drift", Path: "go.mod", Line: 1},
			{Plugin: "repo", Rule: "mod/not-tidy", Severity: "warning", Category: "mod", Message: "go.mod drift", Path: "go.mod", Line: 1, Suppressed: true},
		},
	}

	body := string(Render(result, Options{UseUnicode: false}))
	activeIndex := strings.Index(body, "Active Findings")
	suppressedIndex := strings.Index(body, "Suppressed Findings")
	if activeIndex == -1 || suppressedIndex == -1 || suppressedIndex <= activeIndex {
		t.Fatalf("expected active and suppressed sections in order, got %q", body)
	}
}

func TestRenderShowsToolStatus(t *testing.T) {
	result := godoctor.DiagnoseResult{
		Project:      godoctor.ProjectInfo{Root: "/repo", Mode: "module", GoVersion: "1.23.0", ModuleRoots: []string{"/repo"}, PackageCount: 1},
		SkippedTools: []string{"govulncheck (diff default)"},
		ToolErrors:   []godoctor.ToolError{{Tool: "diff", Message: "running full scan"}},
	}

	body := string(Render(result, Options{UseUnicode: false}))
	if !strings.Contains(body, "skipped: govulncheck (diff default)") {
		t.Fatalf("expected skipped tool output, got %q", body)
	}
	if !strings.Contains(body, "tool diff: running full scan") {
		t.Fatalf("expected tool error output, got %q", body)
	}
}
