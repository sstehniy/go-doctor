package thirdparty

import (
	"path/filepath"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
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
