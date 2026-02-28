package thirdparty

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

type goVetAdapter struct{}

func newGoVetAdapter() diagnostics.Analyzer {
	return goVetAdapter{}
}

func (goVetAdapter) Name() string {
	return "govet"
}

func (goVetAdapter) SupportsDiff() bool {
	return true
}

func (goVetAdapter) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	args := append([]string{"vet"}, packagePatterns(target)...)
	result, err := runCommand(ctx, target.RepoRoot, "go", args...)
	if err != nil {
		return toolFailure("govet", missingToolError("go", "https://go.dev/dl/", err))
	}
	text := combinedOutput(result)
	if result.exitCode != 0 && text == "" {
		return toolFailure("govet", fmt.Errorf("go vet exited with code %d", result.exitCode))
	}
	return diagnostics.Result{
		Metadata:    diagnostics.AnalyzerMetadata{Name: "govet", Scope: "package"},
		Diagnostics: filterGeneratedDiagnostics(target, parseGoVetOutput(text, target.RepoRoot)),
	}
}

func parseGoVetOutput(output string, repoRoot string) []model.Diagnostic {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	var diagnosticsOut []model.Diagnostic
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		diag, ok := parseCompilerStyleDiagnostic(line, repoRoot)
		if !ok {
			continue
		}
		diag.Plugin = "govet"
		diag.Rule = normalizeGoVetRule(diag.Message)
		diag.Severity = goVetSeverity(diag.Rule)
		diag.Category = goVetCategory(diag.Rule)
		diagnosticsOut = append(diagnosticsOut, diag)
	}
	return diagnosticsOut
}

func parseCompilerStyleDiagnostic(line string, repoRoot string) (model.Diagnostic, bool) {
	messageSep := strings.LastIndex(line, ":")
	if messageSep < 0 {
		return model.Diagnostic{}, false
	}
	columnSep := strings.LastIndex(line[:messageSep], ":")
	if columnSep < 0 {
		return model.Diagnostic{}, false
	}
	lineSep := strings.LastIndex(line[:columnSep], ":")
	if lineSep < 0 {
		return model.Diagnostic{}, false
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(line[lineSep+1 : columnSep]))
	if err != nil {
		return model.Diagnostic{}, false
	}
	columnNumber, err := strconv.Atoi(strings.TrimSpace(line[columnSep+1 : messageSep]))
	if err != nil {
		return model.Diagnostic{}, false
	}
	path := normalizeDiagnosticPath(line[:lineSep], repoRoot)
	return model.Diagnostic{
		Path:    path,
		Line:    lineNumber,
		Column:  columnNumber,
		Message: strings.TrimSpace(line[messageSep+1:]),
	}, true
}

func normalizeGoVetRule(message string) string {
	known := []string{
		"printf",
		"copylocks",
		"lostcancel",
		"loopclosure",
		"shift",
		"unmarshal",
		"nilfunc",
		"unreachable",
	}
	lower := strings.ToLower(message)
	for _, rule := range known {
		if strings.Contains(lower, rule) {
			return rule
		}
	}
	return "govet"
}

func goVetSeverity(rule string) string {
	switch rule {
	case "printf", "copylocks", "lostcancel", "loopclosure", "shift", "unmarshal", "nilfunc", "unreachable":
		return "error"
	default:
		return "warning"
	}
}

func goVetCategory(rule string) string {
	switch rule {
	case "govet":
		return "reliability"
	default:
		return "correctness"
	}
}

func normalizeDiagnosticPath(path string, repoRoot string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		if relative, err := filepath.Rel(repoRoot, cleaned); err == nil {
			return model.NormalizePath(relative)
		}
	}
	return model.NormalizePath(cleaned)
}
