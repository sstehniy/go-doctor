package thirdparty

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

type staticcheckAdapter struct{}

func newStaticcheckAdapter() diagnostics.Analyzer {
	return staticcheckAdapter{}
}

func (staticcheckAdapter) Name() string {
	return "staticcheck"
}

func (staticcheckAdapter) SupportsDiff() bool {
	return true
}

func (staticcheckAdapter) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	args := append([]string{"-f", "json"}, packagePatterns(target)...)
	result, err := runCommand(ctx, target.RepoRoot, "staticcheck", args...)
	if err != nil {
		return toolFailure("staticcheck", missingToolError("staticcheck", "go install honnef.co/go/tools/cmd/staticcheck@latest", err))
	}
	diagnosticsOut := parseStaticcheckJSON(result.stdout, target.RepoRoot)
	if len(diagnosticsOut) == 0 {
		diagnosticsOut = parseStaticcheckText(combinedOutput(result), target.RepoRoot)
	}
	if result.exitCode != 0 && len(diagnosticsOut) == 0 {
		return toolFailure("staticcheck", fmt.Errorf("staticcheck exited with code %d", result.exitCode))
	}
	return diagnostics.Result{
		Metadata:    diagnostics.AnalyzerMetadata{Name: "staticcheck", Scope: "package"},
		Diagnostics: filterGeneratedDiagnostics(target, diagnosticsOut),
	}
}

type staticcheckLine struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Location struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	} `json:"location"`
	End struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"end"`
}

func parseStaticcheckJSON(output string, repoRoot string) []model.Diagnostic {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	var diagnosticsOut []model.Diagnostic
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var finding staticcheckLine
		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			continue
		}
		diagnosticsOut = append(diagnosticsOut, model.Diagnostic{
			Path:      normalizeDiagnosticPath(finding.Location.File, repoRoot),
			Line:      finding.Location.Line,
			Column:    finding.Location.Column,
			EndLine:   finding.End.Line,
			EndColumn: finding.End.Column,
			Plugin:    "staticcheck",
			Rule:      finding.Code,
			Severity:  "warning",
			Category:  staticcheckCategory(finding.Code),
			Message:   finding.Message,
		})
	}
	return diagnosticsOut
}

func parseStaticcheckText(output string, repoRoot string) []model.Diagnostic {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(output))
	var diagnosticsOut []model.Diagnostic
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		lineNumber, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		columnNumber, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			continue
		}
		message := strings.TrimSpace(parts[3])
		rule := "staticcheck"
		if start := strings.LastIndex(message, "("); start >= 0 && strings.HasSuffix(message, ")") {
			rule = strings.TrimSpace(message[start+1 : len(message)-1])
			message = strings.TrimSpace(message[:start])
		}
		diagnosticsOut = append(diagnosticsOut, model.Diagnostic{
			Path:     normalizeDiagnosticPath(parts[0], repoRoot),
			Line:     lineNumber,
			Column:   columnNumber,
			Plugin:   "staticcheck",
			Rule:     rule,
			Severity: "warning",
			Category: staticcheckCategory(rule),
			Message:  message,
		})
	}
	return diagnosticsOut
}

func staticcheckCategory(rule string) string {
	switch {
	case strings.HasPrefix(rule, "SA"):
		return "correctness"
	case strings.HasPrefix(rule, "ST"):
		return "maintainability"
	case strings.HasPrefix(rule, "S"):
		return "simplification"
	case strings.HasPrefix(rule, "QF"):
		return "quickfix"
	default:
		return "maintainability"
	}
}
