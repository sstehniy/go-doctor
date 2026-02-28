package thirdparty

import (
	"context"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

type ExecResult = execResult

func ExportDefaultAnalyzers(target diagnostics.Target, enableRules []string, disableRules []string) []diagnostics.Analyzer {
	return DefaultAnalyzers(target, enableRules, disableRules)
}

func ExportParseGoVetOutput(output string, repoRoot string) []model.Diagnostic {
	return parseGoVetOutput(output, repoRoot)
}

func ExportParseStaticcheckJSON(output string, repoRoot string) []model.Diagnostic {
	return parseStaticcheckJSON(output, repoRoot)
}

func ExportParseStaticcheckText(output string, repoRoot string) []model.Diagnostic {
	return parseStaticcheckText(output, repoRoot)
}

func ExportParseGovulncheckJSON(output string, repoRoot string) []model.Diagnostic {
	return parseGovulncheckJSON(output, repoRoot)
}

func ExportParseGolangCIJSON(output string, repoRoot string) ([]model.Diagnostic, error) {
	return parseGolangCIJSON(output, repoRoot)
}

func ExportFilterModuleRoots(moduleRoots []string, patterns []string) []string {
	return filterModuleRoots(moduleRoots, patterns)
}

func ExportPackagePatterns(target diagnostics.Target) []string {
	return packagePatterns(target)
}

func ExportFilterGeneratedDiagnostics(target diagnostics.Target, diagnosticsOut []model.Diagnostic) []model.Diagnostic {
	return filterGeneratedDiagnostics(target, diagnosticsOut)
}

func ExportNewGovulncheckAdapter() diagnostics.Analyzer {
	return newGovulncheckAdapter()
}

func ExportSwapRunCommand(
	fn func(ctx context.Context, dir string, name string, args ...string) (execResult, error),
) func() {
	original := runCommand
	runCommand = fn
	return func() {
		runCommand = original
	}
}
