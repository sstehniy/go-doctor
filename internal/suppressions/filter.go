package suppressions

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
	"golang.org/x/tools/go/packages"
)

const (
	InvalidRule = "suppress/invalid"

	invalidHelp = "Use // godoctor:ignore <rule> <reason> or // godoctor:ignore-next-line <rule> <reason>."
)

type Filter struct {
	byPath map[string][]directive
}

type directive struct {
	path       string
	rule       string
	targetLine int
}

func Load(target diagnostics.Target) (Filter, []model.Diagnostic, []model.ToolError) {
	filter := Filter{byPath: map[string][]directive{}}
	roots := selectedRoots(target)
	if len(roots) == 0 {
		return filter, nil, nil
	}
	selectedFiles, packageToolErrors := selectedFiles(target)

	var diagnosticsOut []model.Diagnostic
	toolErrors := append([]model.ToolError(nil), packageToolErrors...)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if skipDir(path, root) {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			if len(selectedFiles) > 0 {
				if _, ok := selectedFiles[filepath.Clean(path)]; !ok {
					return nil
				}
			}

			fileDirectives, fileDiagnostics, err := parseFile(target, path)
			diagnosticsOut = append(diagnosticsOut, fileDiagnostics...)
			if err != nil {
				toolErrors = append(toolErrors, model.ToolError{
					Tool:    "suppressions",
					Message: fmt.Sprintf("parse %s: %v", path, err),
				})
			}
			for _, directive := range fileDirectives {
				filter.byPath[directive.path] = append(filter.byPath[directive.path], directive)
			}
			return nil
		})
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "suppressions",
				Message: fmt.Sprintf("walk %s: %v", root, err),
			})
		}
	}

	return filter, diagnosticsOut, toolErrors
}

func Apply(diagnosticsOut []model.Diagnostic, filter Filter) []model.Diagnostic {
	if len(diagnosticsOut) == 0 || len(filter.byPath) == 0 {
		return diagnosticsOut
	}

	out := make([]model.Diagnostic, 0, len(diagnosticsOut))
	for _, diagnostic := range diagnosticsOut {
		if !diagnostic.Suppressed && filter.matches(diagnostic) {
			diagnostic.Suppressed = true
		}
		out = append(out, diagnostic)
	}
	return out
}

func (f Filter) matches(diagnostic model.Diagnostic) bool {
	if diagnostic.Path == "" {
		return false
	}

	directives := f.byPath[model.NormalizePath(diagnostic.Path)]
	if len(directives) == 0 {
		return false
	}

	line := diagnostic.Line
	endLine := diagnostic.EndLine
	if endLine == 0 {
		endLine = line
	}
	for _, directive := range directives {
		if directive.rule != diagnostic.Rule && directive.rule != diagnostic.Plugin {
			continue
		}
		if directive.targetLine >= line && directive.targetLine <= endLine {
			return true
		}
	}
	return false
}

func selectedRoots(target diagnostics.Target) []string {
	roots := target.ModuleRoots
	if len(target.ModulePatterns) > 0 {
		roots = filterModuleRoots(roots, target.ModulePatterns)
		if len(roots) == 0 {
			return nil
		}
	}
	if len(roots) == 0 {
		roots = []string{target.RepoRoot}
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		cleaned := filepath.Clean(absRoot)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	slices.Sort(out)
	return out
}

func selectedFiles(target diagnostics.Target) (map[string]struct{}, []model.ToolError) {
	if len(target.PackagePatterns) == 0 {
		return nil, nil
	}

	config := &packages.Config{
		Dir:   target.RepoRoot,
		Mode:  packages.NeedFiles | packages.NeedCompiledGoFiles,
		Tests: true,
	}
	pkgs, err := packages.Load(config, target.PackagePatterns...)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "suppressions",
			Message: fmt.Sprintf("load packages: %v", err),
		}}
	}

	selected := map[string]struct{}{}
	for _, pkg := range pkgs {
		for _, path := range pkg.GoFiles {
			selected[filepath.Clean(path)] = struct{}{}
		}
		for _, path := range pkg.CompiledGoFiles {
			selected[filepath.Clean(path)] = struct{}{}
		}
	}
	return selected, nil
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

func skipDir(path string, moduleRoot string) bool {
	if filepath.Clean(path) == filepath.Clean(moduleRoot) {
		return false
	}
	name := filepath.Base(path)
	if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

func parseFile(target diagnostics.Target, path string) ([]directive, []model.Diagnostic, error) {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.AllErrors)
	if fileNode == nil {
		return nil, nil, err
	}
	if ast.IsGenerated(fileNode) && !target.IncludeGenerated {
		return nil, nil, nil
	}

	relPath, relErr := filepath.Rel(target.RepoRoot, path)
	if relErr != nil {
		return nil, nil, fmt.Errorf("resolve path %s: %w", path, relErr)
	}
	normalizedPath := model.NormalizePath(relPath)
	isTestFile := strings.HasSuffix(normalizedPath, "_test.go")

	var directives []directive
	var diagnosticsOut []model.Diagnostic
	for _, group := range fileNode.Comments {
		for _, comment := range group.List {
			parsed, invalid, ok := parseDirective(normalizedPath, comment, fset, isTestFile)
			if invalid != nil {
				diagnosticsOut = append(diagnosticsOut, *invalid)
			}
			if ok {
				directives = append(directives, parsed)
			}
		}
	}

	return directives, diagnosticsOut, nil
}

func parseDirective(path string, comment *ast.Comment, fset *token.FileSet, isTestFile bool) (directive, *model.Diagnostic, bool) {
	if comment == nil || !strings.HasPrefix(comment.Text, "//") {
		return directive{}, nil, false
	}

	text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
	if !strings.HasPrefix(text, "godoctor:") {
		return directive{}, nil, false
	}

	position := fset.PositionFor(comment.Slash, false)
	body := strings.TrimSpace(strings.TrimPrefix(text, "godoctor:"))
	name, rest := cutField(body)
	if name == "" {
		diagnostic := invalidDiagnostic(path, position.Line, position.Column, "suppression directive is missing a command")
		return directive{}, &diagnostic, false
	}

	kind := ""
	switch name {
	case "ignore", "ignore-next-line":
		kind = name
	default:
		diagnostic := invalidDiagnostic(path, position.Line, position.Column, fmt.Sprintf("unknown suppression directive %q", "godoctor:"+name))
		return directive{}, &diagnostic, false
	}

	rule, reason := cutField(rest)
	if rule == "" {
		diagnostic := invalidDiagnostic(path, position.Line, position.Column, "suppression directive is missing a rule name")
		return directive{}, &diagnostic, false
	}
	if !isTestFile && strings.TrimSpace(reason) == "" {
		diagnostic := invalidDiagnostic(path, position.Line, position.Column, fmt.Sprintf("suppression for %q requires a reason outside test files", rule))
		return directive{}, &diagnostic, false
	}

	targetLine := position.Line
	if kind == "ignore-next-line" {
		targetLine++
	}
	return directive{
		path:       path,
		rule:       rule,
		targetLine: targetLine,
	}, nil, true
}

func cutField(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	for index, r := range value {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return value[:index], strings.TrimSpace(value[index+1:])
		}
	}
	return value, ""
}

func invalidDiagnostic(path string, line int, column int, message string) model.Diagnostic {
	return model.Diagnostic{
		Path:      path,
		Line:      line,
		Column:    column,
		EndLine:   line,
		EndColumn: column,
		Plugin:    "repo",
		Rule:      InvalidRule,
		Severity:  "warning",
		Category:  "maintainability",
		Message:   message,
		Help:      invalidHelp,
	}
}
