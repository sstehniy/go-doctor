package thirdparty

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type golangciAdapter struct {
	linters []string
}

func newGolangCIAdapter(linters []string) diagnostics.Analyzer {
	copied := make([]string, len(linters))
	copy(copied, linters)
	return golangciAdapter{linters: copied}
}

func (g golangciAdapter) Name() string {
	return "golangci-lint"
}

func (golangciAdapter) SupportsDiff() bool {
	return true
}

func (g golangciAdapter) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	args := []string{"run", "--out-format", "json", "--disable-all"}
	for _, linter := range g.linters {
		args = append(args, "--enable", linter)
	}
	args = append(args, packagePatterns(target)...)
	result, err := runCommand(ctx, target.RepoRoot, "golangci-lint", args...)
	if err != nil {
		return toolFailure("golangci-lint", missingToolError("golangci-lint", "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest", err))
	}

	diagnosticsOut, parseErr := parseGolangCIJSON(result.stdout, target.RepoRoot)
	if parseErr != nil {
		return toolFailure("golangci-lint", parseErr)
	}
	if result.exitCode != 0 && len(diagnosticsOut) == 0 {
		return toolFailure("golangci-lint", fmt.Errorf("golangci-lint exited with code %d", result.exitCode))
	}
	return diagnostics.Result{
		Metadata:    diagnostics.AnalyzerMetadata{Name: "golangci-lint", Scope: "package"},
		Diagnostics: filterGeneratedDiagnostics(target, diagnosticsOut),
	}
}

type golangciPayload struct {
	Issues []struct {
		FromLinter string `json:"FromLinter"`
		Text       string `json:"Text"`
		Pos        struct {
			Filename string `json:"Filename"`
			Line     int    `json:"Line"`
			Column   int    `json:"Column"`
		} `json:"Pos"`
	} `json:"Issues"`
}

func parseGolangCIJSON(output string, repoRoot string) ([]model.Diagnostic, error) {
	if output == "" {
		return nil, nil
	}
	var payload golangciPayload
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, fmt.Errorf("parse golangci-lint output: %w", err)
	}
	diagnosticsOut := make([]model.Diagnostic, 0, len(payload.Issues))
	for _, issue := range payload.Issues {
		diagnosticsOut = append(diagnosticsOut, model.Diagnostic{
			Path:     normalizeDiagnosticPath(issue.Pos.Filename, repoRoot),
			Line:     issue.Pos.Line,
			Column:   issue.Pos.Column,
			Plugin:   "golangci-lint",
			Rule:     issue.FromLinter,
			Severity: golangciSeverity(issue.FromLinter, issue.Text, issue.Pos.Filename),
			Category: golangciCategory(issue.FromLinter),
			Message:  issue.Text,
		})
	}
	return diagnosticsOut, nil
}

func golangciSeverity(rule string, message string, filename string) string {
	switch rule {
	case "sqlclosecheck":
		return "error"
	case "rowserrcheck":
		if containsRowsLoop(message) && !strings.HasSuffix(strings.ToLower(filename), "_test.go") {
			return "error"
		}
		return "warning"
	case "exportloopref":
		return "error"
	default:
		return "warning"
	}
}

func golangciCategory(rule string) string {
	switch rule {
	case "sqlclosecheck", "rowserrcheck", "bodyclose":
		return "reliability"
	case "exportloopref":
		return "correctness"
	case "prealloc":
		return "performance"
	default:
		return "maintainability"
	}
}

func containsRowsLoop(message string) bool {
	return containsAll(message, "rows", "loop")
}
