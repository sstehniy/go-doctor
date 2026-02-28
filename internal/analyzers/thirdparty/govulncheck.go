package thirdparty

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type govulncheckAdapter struct{}

func newGovulncheckAdapter() diagnostics.Analyzer {
	return govulncheckAdapter{}
}

func (govulncheckAdapter) Name() string {
	return "govulncheck"
}

func (govulncheckAdapter) SupportsDiff() bool {
	return false
}

func (govulncheckAdapter) Run(ctx context.Context, target diagnostics.Target) diagnostics.Result {
	moduleRoots := target.ModuleRoots
	if len(moduleRoots) == 0 {
		moduleRoots = []string{target.RepoRoot}
	}
	if len(target.ModulePatterns) > 0 {
		moduleRoots = filterModuleRoots(moduleRoots, target.ModulePatterns)
	}
	if len(moduleRoots) == 0 {
		return diagnostics.Result{
			Metadata: diagnostics.AnalyzerMetadata{Name: "govulncheck", Scope: "module"},
			ToolErrors: []model.ToolError{
				{Tool: "govulncheck", Message: "no module roots matched requested module filters"},
			},
		}
	}

	combined := diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{Name: "govulncheck", Scope: "module"},
	}
	for _, moduleRoot := range moduleRoots {
		result, err := runCommand(ctx, moduleRoot, "govulncheck", "-json", "./...")
		if err != nil {
			return toolFailure("govulncheck", missingToolError("govulncheck", "go install golang.org/x/vuln/cmd/govulncheck@latest", err))
		}
		diagnosticsOut := parseGovulncheckJSON(result.stdout, target.RepoRoot)
		if result.exitCode != 0 && len(diagnosticsOut) == 0 {
			return toolFailure("govulncheck", fmt.Errorf("govulncheck exited with code %d", result.exitCode))
		}
		combined.Diagnostics = append(combined.Diagnostics, filterGeneratedDiagnostics(target, diagnosticsOut)...)
	}
	return combined
}

type govulncheckMessage struct {
	OSV *struct {
		ID      string `json:"id"`
		Details string `json:"details"`
	} `json:"osv"`
	Finding *govulncheckFinding `json:"finding"`
}

type govulncheckFinding struct {
	OSV          string `json:"osv"`
	FixedVersion string `json:"fixed_version"`
	Trace        []struct {
		Module   string `json:"module"`
		Package  string `json:"package"`
		Function string `json:"function"`
		Position *struct {
			Filename string `json:"filename"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"position"`
	} `json:"trace"`
}

func parseGovulncheckJSON(output string, repoRoot string) []model.Diagnostic {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	descriptions := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	var diagnosticsOut []model.Diagnostic
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var message govulncheckMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			continue
		}
		if message.OSV != nil {
			descriptions[message.OSV.ID] = strings.TrimSpace(message.OSV.Details)
			continue
		}
		if message.Finding == nil {
			continue
		}
		diagnosticsOut = append(diagnosticsOut, normalizeGovulnFinding(*message.Finding, descriptions, repoRoot))
	}
	return diagnosticsOut
}

func normalizeGovulnFinding(finding govulncheckFinding, descriptions map[string]string, repoRoot string) model.Diagnostic {
	diag := model.Diagnostic{
		Plugin:   "govulncheck",
		Rule:     finding.OSV,
		Category: "security",
		Message:  "vulnerability detected",
		Severity: "warning",
	}
	if detail := descriptions[finding.OSV]; detail != "" {
		diag.Message = detail
	}
	if finding.FixedVersion != "" {
		diag.Help = "upgrade to " + finding.FixedVersion
	}

	reachable := false
	for _, frame := range finding.Trace {
		if diag.Module == "" {
			diag.Module = frame.Module
		}
		if diag.Package == "" {
			diag.Package = frame.Package
		}
		if frame.Function != "" {
			reachable = true
		}
		if frame.Position != nil && diag.Path == "" {
			diag.Path = normalizeDiagnosticPath(frame.Position.Filename, repoRoot)
			diag.Line = frame.Position.Line
			diag.Column = frame.Position.Column
		}
	}
	if reachable {
		diag.Severity = "error"
	}
	return diag
}
