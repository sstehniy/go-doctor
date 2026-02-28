package thirdparty

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

func toolFailure(name string, err error) diagnostics.Result {
	return diagnostics.Result{
		Metadata: diagnostics.AnalyzerMetadata{Name: name},
		ToolErrors: []model.ToolError{
			{
				Tool:    name,
				Message: err.Error(),
			},
		},
	}
}

func packagePatterns(target diagnostics.Target) []string {
	if len(target.PackagePatterns) > 0 {
		patterns := make([]string, len(target.PackagePatterns))
		copy(patterns, target.PackagePatterns)
		return patterns
	}
	if len(target.ModulePatterns) > 0 {
		moduleRoots := filterModuleRoots(target.ModuleRoots, target.ModulePatterns)
		if len(moduleRoots) > 0 {
			patterns := make([]string, 0, len(moduleRoots))
			for _, root := range moduleRoots {
				relative, err := filepath.Rel(target.RepoRoot, root)
				if err != nil {
					continue
				}
				normalized := model.NormalizePath(relative)
				if normalized == "." {
					patterns = append(patterns, "./...")
					continue
				}
				patterns = append(patterns, "./"+normalized+"/...")
			}
			if len(patterns) > 0 {
				return patterns
			}
		}
	}
	return []string{"./..."}
}

func filterGeneratedDiagnostics(target diagnostics.Target, diagnosticsOut []model.Diagnostic) []model.Diagnostic {
	if target.IncludeGenerated || len(diagnosticsOut) == 0 {
		return diagnosticsOut
	}

	filtered := diagnosticsOut[:0]
	generatedByPath := make(map[string]bool, len(diagnosticsOut))
	for _, diag := range diagnosticsOut {
		if diag.Path == "" {
			filtered = append(filtered, diag)
			continue
		}
		generated, ok := generatedByPath[diag.Path]
		if !ok {
			generated = isGeneratedFile(target.RepoRoot, diag.Path)
			generatedByPath[diag.Path] = generated
		}
		if generated {
			continue
		}
		filtered = append(filtered, diag)
	}
	return filtered
}

func filterModuleRoots(moduleRoots []string, patterns []string) []string {
	var filtered []string
	for _, root := range moduleRoots {
		normalizedRoot := model.NormalizePath(root)
		base := filepath.Base(root)
		for _, pattern := range patterns {
			normalizedPattern := model.NormalizePath(pattern)
			if normalizedPattern == normalizedRoot || normalizedPattern == base || strings.HasSuffix(normalizedRoot, "/"+normalizedPattern) {
				filtered = append(filtered, root)
				break
			}
		}
	}
	return filtered
}

func containsAll(value string, parts ...string) bool {
	lower := strings.ToLower(value)
	for _, part := range parts {
		if !strings.Contains(lower, strings.ToLower(part)) {
			return false
		}
	}
	return true
}

func isGeneratedFile(repoRoot string, diagnosticPath string) bool {
	absPath := filepath.Clean(diagnosticPath)
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(repoRoot, filepath.FromSlash(diagnosticPath))
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return false
	}
	fileNode, err := parser.ParseFile(token.NewFileSet(), absPath, nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		return false
	}
	return ast.IsGenerated(fileNode)
}
