package thirdparty_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stanislavstehniy/go-doctor/internal/analyzers/thirdparty"
	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

func TestDefaultAnalyzers(t *testing.T) {
	t.Run("defaults include exportloopref before go122", func(t *testing.T) {
		analyzers := thirdparty.ExportDefaultAnalyzers(diagnostics.Target{GoVersion: "1.21"}, nil, nil)
		if len(analyzers) != 4 {
			t.Fatalf("expected 4 analyzers, got %d", len(analyzers))
		}
		if analyzers[3].Name() != "golangci-lint" {
			t.Fatalf("expected golangci adapter, got %T", analyzers[3])
		}
	})

	t.Run("linter-only enable builds narrowed golangci adapter", func(t *testing.T) {
		analyzers := thirdparty.ExportDefaultAnalyzers(diagnostics.Target{GoVersion: "1.22"}, []string{"sqlclosecheck"}, nil)
		if len(analyzers) != 1 {
			t.Fatalf("expected 1 analyzer, got %d", len(analyzers))
		}
		if analyzers[0].Name() != "golangci-lint" {
			t.Fatalf("expected golangci adapter, got %T", analyzers[0])
		}
	})
}

func TestParseGoVetOutput(t *testing.T) {
	output := "/repo/main.go:10:4: printf format %d has arg x of wrong type string\n"
	diagnosticsOut := thirdparty.ExportParseGoVetOutput(output, "/repo")
	if len(diagnosticsOut) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
	}
	diag := diagnosticsOut[0]
	if diag.Path != "main.go" {
		t.Fatalf("expected normalized path, got %q", diag.Path)
	}
	if diag.Rule != "printf" {
		t.Fatalf("expected printf rule, got %q", diag.Rule)
	}
	if diag.Severity != "error" {
		t.Fatalf("expected error severity, got %q", diag.Severity)
	}
}

func TestParseGoVetOutputWindowsPath(t *testing.T) {
	output := "C:\\repo\\main.go:10:4: printf format %d has arg x of wrong type string\n"
	diagnosticsOut := thirdparty.ExportParseGoVetOutput(output, "C:\\repo")
	if len(diagnosticsOut) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
	}
	if diagnosticsOut[0].Line != 10 || diagnosticsOut[0].Column != 4 {
		t.Fatalf("unexpected position: %d:%d", diagnosticsOut[0].Line, diagnosticsOut[0].Column)
	}
}

func TestParseStaticcheck(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		output := `{"code":"SA1000","message":"bad regexp","location":{"file":"/repo/main.go","line":8,"column":2},"end":{"line":8,"column":6}}`
		diagnosticsOut := thirdparty.ExportParseStaticcheckJSON(output, "/repo")
		if len(diagnosticsOut) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
		}
		if diagnosticsOut[0].Category != "correctness" {
			t.Fatalf("expected correctness, got %q", diagnosticsOut[0].Category)
		}
		if diagnosticsOut[0].Rule != "SA1000" {
			t.Fatalf("expected SA1000, got %q", diagnosticsOut[0].Rule)
		}
	})

	t.Run("text fallback", func(t *testing.T) {
		output := "/repo/main.go:8:2: bad regexp (QF1001)"
		diagnosticsOut := thirdparty.ExportParseStaticcheckText(output, "/repo")
		if len(diagnosticsOut) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
		}
		if diagnosticsOut[0].Category != "quickfix" {
			t.Fatalf("expected quickfix, got %q", diagnosticsOut[0].Category)
		}
	})
}

func TestParseGovulncheckJSON(t *testing.T) {
	output := `{"osv":{"id":"GO-2024-0001","details":"reachable vuln"}}` + "\n" +
		`{"finding":{"osv":"GO-2024-0001","fixed_version":"v1.2.3","trace":[{"module":"example.com/mod","package":"example.com/mod/pkg","function":"pkg.Run","position":{"filename":"/repo/main.go","line":12,"column":3}}]}}`
	diagnosticsOut := thirdparty.ExportParseGovulncheckJSON(output, "/repo")
	if len(diagnosticsOut) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
	}
	diag := diagnosticsOut[0]
	if diag.Severity != "error" {
		t.Fatalf("expected reachable vuln to be error, got %q", diag.Severity)
	}
	if diag.Help != "upgrade to v1.2.3" {
		t.Fatalf("unexpected help: %q", diag.Help)
	}
	if diag.Path != "main.go" {
		t.Fatalf("unexpected path: %q", diag.Path)
	}
}

func TestParseGolangCIJSON(t *testing.T) {
	output := `{"Issues":[` +
		`{"FromLinter":"sqlclosecheck","Text":"sql.Rows.Close must be checked","Pos":{"Filename":"/repo/db.go","Line":5,"Column":1}},` +
		`{"FromLinter":"rowserrcheck","Text":"check rows.Err in rows loop","Pos":{"Filename":"/repo/db.go","Line":9,"Column":2}}]}`
	diagnosticsOut, err := thirdparty.ExportParseGolangCIJSON(output, "/repo")
	if err != nil {
		t.Fatalf("parse golangci json: %v", err)
	}
	if len(diagnosticsOut) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diagnosticsOut))
	}
	if diagnosticsOut[0].Severity != "error" {
		t.Fatalf("expected sqlclosecheck error, got %q", diagnosticsOut[0].Severity)
	}
	if diagnosticsOut[1].Severity != "error" {
		t.Fatalf("expected promoted rowserrcheck error, got %q", diagnosticsOut[1].Severity)
	}
}

func TestParseGolangCIJSONKeepsTestRowserrcheckAsWarning(t *testing.T) {
	output := `{"Issues":[{"FromLinter":"rowserrcheck","Text":"check rows.Err in rows loop","Pos":{"Filename":"/repo/db_test.go","Line":9,"Column":2}}]}`
	diagnosticsOut, err := thirdparty.ExportParseGolangCIJSON(output, "/repo")
	if err != nil {
		t.Fatalf("parse golangci json: %v", err)
	}
	if len(diagnosticsOut) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnosticsOut))
	}
	if diagnosticsOut[0].Severity != "warning" {
		t.Fatalf("expected test rowserrcheck warning, got %q", diagnosticsOut[0].Severity)
	}
}

func TestFilterModuleRoots(t *testing.T) {
	moduleRoots := []string{"/repo/moda", "/repo/modb"}
	filtered := thirdparty.ExportFilterModuleRoots(moduleRoots, []string{"modb"})
	if len(filtered) != 1 || filtered[0] != "/repo/modb" {
		t.Fatalf("unexpected filtered modules: %#v", filtered)
	}
}

func TestPackagePatternsUsesModuleFilters(t *testing.T) {
	patterns := thirdparty.ExportPackagePatterns(diagnostics.Target{
		RepoRoot:       "/repo",
		ModuleRoots:    []string{"/repo/moda", "/repo/modb"},
		ModulePatterns: []string{"modb"},
	})
	if len(patterns) != 1 || patterns[0] != "./modb/..." {
		t.Fatalf("unexpected patterns: %#v", patterns)
	}
}

func TestGovulncheckRunsPerFilteredModule(t *testing.T) {
	var dirs []string
	restore := thirdparty.ExportSwapRunCommand(func(ctx context.Context, dir string, name string, args ...string) (thirdparty.ExecResult, error) {
		dirs = append(dirs, dir)
		return thirdparty.ExecResult{}, nil
	})
	defer restore()

	adapter := thirdparty.ExportNewGovulncheckAdapter()
	result := adapter.Run(context.Background(), diagnostics.Target{
		RepoRoot:       "/repo",
		ModuleRoots:    []string{"/repo/moda", "/repo/modb"},
		ModulePatterns: []string{"modb"},
	})

	if len(result.ToolErrors) != 0 {
		t.Fatalf("unexpected tool errors: %#v", result.ToolErrors)
	}
	if !slices.Equal(dirs, []string{"/repo/modb"}) {
		t.Fatalf("unexpected dirs: %#v", dirs)
	}
}

func TestFilterGeneratedDiagnostics(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "generated.go"), "// Code generated by test. DO NOT EDIT.\npackage repo\n")
	writeFile(t, filepath.Join(repoRoot, "main.go"), "package repo\n")

	diagnosticsOut := []model.Diagnostic{
		{Path: "generated.go", Plugin: "govet", Rule: "printf"},
		{Path: "main.go", Plugin: "govet", Rule: "printf"},
		{Plugin: "govet", Rule: "printf"},
	}

	filtered := thirdparty.ExportFilterGeneratedDiagnostics(diagnostics.Target{RepoRoot: repoRoot}, diagnosticsOut)
	if len(filtered) != 2 {
		t.Fatalf("expected generated diagnostic to be filtered, got %#v", filtered)
	}
	if filtered[0].Path != "main.go" {
		t.Fatalf("expected nongenerated file to remain, got %#v", filtered)
	}
	if filtered[1].Path != "" {
		t.Fatalf("expected pathless diagnostic to remain, got %#v", filtered)
	}
}

func TestFilterGeneratedDiagnosticsIncludesGeneratedWhenEnabled(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "generated.go"), "// Code generated by test. DO NOT EDIT.\npackage repo\n")

	diagnosticsOut := []model.Diagnostic{{Path: "generated.go", Plugin: "govet", Rule: "printf"}}
	filtered := thirdparty.ExportFilterGeneratedDiagnostics(diagnostics.Target{
		RepoRoot:         repoRoot,
		IncludeGenerated: true,
	}, diagnosticsOut)
	if len(filtered) != 1 {
		t.Fatalf("expected generated diagnostic to remain when enabled, got %#v", filtered)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
