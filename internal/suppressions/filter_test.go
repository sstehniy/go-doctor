package suppressions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

func TestApplyMarksMatchingDiagnosticsSuppressed(t *testing.T) {
	root := writeRepo(t, "main.go", "package main\nfunc main() {\n\tprintln(\"hi\") // godoctor:ignore custom/rule legacy\n}\n")

	filter, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	})
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid directives, got %#v", invalid)
	}
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}

	diagnosticsOut := Apply([]model.Diagnostic{{
		Path:      "main.go",
		Line:      3,
		Column:    2,
		EndLine:   3,
		EndColumn: 16,
		Plugin:    "custom",
		Rule:      "custom/rule",
		Severity:  "warning",
		Message:   "flagged",
	}}, filter)
	if !diagnosticsOut[0].Suppressed {
		t.Fatal("expected diagnostic to be suppressed")
	}
}

func TestLoadReportsMissingReasonOutsideTests(t *testing.T) {
	root := writeRepo(t, "main.go", "package main\nfunc main() {\n\tprintln(\"hi\") // godoctor:ignore custom/rule\n}\n")

	_, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	})
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}
	if len(invalid) != 1 {
		t.Fatalf("expected one invalid directive diagnostic, got %#v", invalid)
	}
	if invalid[0].Rule != InvalidRule {
		t.Fatalf("expected %q rule, got %#v", InvalidRule, invalid[0])
	}
	if invalid[0].Path != "main.go" || invalid[0].Line != 3 {
		t.Fatalf("unexpected invalid directive location: %#v", invalid[0])
	}
}

func TestLoadAllowsMissingReasonInTests(t *testing.T) {
	root := writeRepo(t, "main_test.go", "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {\n\tprintln(\"hi\") // godoctor:ignore custom/rule\n}\n")

	filter, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	})
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid directives, got %#v", invalid)
	}
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}

	diagnosticsOut := Apply([]model.Diagnostic{{
		Path:      "main_test.go",
		Line:      4,
		Column:    2,
		EndLine:   4,
		EndColumn: 16,
		Plugin:    "custom",
		Rule:      "custom/rule",
		Severity:  "warning",
		Message:   "flagged",
	}}, filter)
	if !diagnosticsOut[0].Suppressed {
		t.Fatal("expected test-file directive to suppress without a reason")
	}
}

func TestApplyMatchesPluginNameForDynamicRules(t *testing.T) {
	root := writeRepo(t, "main.go", "package main\nfunc main() {\n\tprintln(\"hi\") // godoctor:ignore staticcheck legacy false positive\n}\n")

	filter, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	})
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid directives, got %#v", invalid)
	}
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}

	diagnosticsOut := Apply([]model.Diagnostic{{
		Path:      "main.go",
		Line:      3,
		Column:    2,
		EndLine:   3,
		EndColumn: 16,
		Plugin:    "staticcheck",
		Rule:      "SA1000",
		Severity:  "warning",
		Message:   "flagged",
	}}, filter)
	if !diagnosticsOut[0].Suppressed {
		t.Fatal("expected plugin-scoped directive to suppress dynamic rule")
	}
}

func TestLoadHonorsPackagePatterns(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/test\n\ngo 1.22.0\n")
	writeFixtureFile(t, root, "pkg/selected/selected.go", "package selected\nfunc Run() {\n\tprintln(\"ok\") // godoctor:ignore custom/rule legacy\n}\n")
	writeFixtureFile(t, root, "pkg/ignored/ignored.go", "package ignored\nfunc Run() {\n\tprintln(\"bad\") // godoctor:ignore custom/rule\n}\n")

	filter, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:        root,
		Mode:            "module",
		ModuleRoots:     []string{root},
		PackagePatterns: []string{"./pkg/selected"},
	})
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}
	if len(invalid) != 0 {
		t.Fatalf("expected ignored package directives to stay out of scope, got %#v", invalid)
	}

	applied := Apply([]model.Diagnostic{{
		Path:      "pkg/selected/selected.go",
		Line:      3,
		Column:    2,
		EndLine:   3,
		EndColumn: 16,
		Plugin:    "custom",
		Rule:      "custom/rule",
		Severity:  "warning",
		Message:   "flagged",
	}}, filter)
	if !applied[0].Suppressed {
		t.Fatal("expected selected package directive to suppress")
	}
}

func TestLoadSkipsScanningWhenNoModulePatternsMatch(t *testing.T) {
	root := writeRepo(t, "main.go", "package main\nfunc main() {\n\tprintln(\"hi\") // godoctor:ignore custom/rule\n}\n")

	filter, invalid, toolErrors := Load(diagnostics.Target{
		RepoRoot:       root,
		Mode:           "module",
		ModuleRoots:    []string{root},
		ModulePatterns: []string{"missing"},
	})
	if len(toolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", toolErrors)
	}
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid directives, got %#v", invalid)
	}

	applied := Apply([]model.Diagnostic{{
		Path:      "main.go",
		Line:      3,
		Column:    2,
		EndLine:   3,
		EndColumn: 16,
		Plugin:    "custom",
		Rule:      "custom/rule",
		Severity:  "warning",
		Message:   "flagged",
	}}, filter)
	if applied[0].Suppressed {
		t.Fatal("expected unmatched module filter to leave diagnostics untouched")
	}
}

func writeRepo(t *testing.T, name string, body string) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/test\n\ngo 1.22.0\n")
	writeFixtureFile(t, root, name, body)
	return root
}

func writeFixtureFile(t *testing.T, root string, name string, body string) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
