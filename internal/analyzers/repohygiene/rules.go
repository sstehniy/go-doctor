package repohygiene

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
	"github.com/sstehniy/go-doctor/internal/model"
)

var registry = []rule{
	defineRule("mod/not-tidy", "mod", true, "warning", "go mod tidy drift", "Run go mod tidy and commit the resulting go.mod/go.sum changes.", runModNotTidy),
	defineRule("mod/replace-local-path", "mod", true, "warning", "Local replace directive", "Avoid committing local filesystem replace directives; prefer versioned modules or go.work during local development.", runReplaceLocalPath),
	defineRule("build/mod-readonly-failure", "build", true, "error", "Readonly module resolution failure", "Ensure dependency metadata is complete so go commands succeed with -mod=readonly.", runModReadonlyFailure),
	defineRule("fmt/not-gofmt", "fmt", false, "info", "File is not gofmt formatted", "Run gofmt on the reported files before committing.", runNotGofmt),
	defineRule("license/missing", "license", false, "info", "Repository is missing a license file", "Add a repository license file such as LICENSE or LICENSE.md.", runMissingLicense),
}

type analysisContext struct {
	target       diagnostics.Target
	repoRoot     string
	moduleRoots  []moduleInfo
	workspaceRel string
}

type moduleInfo struct {
	root       string
	relRoot    string
	modulePath string
}

type execResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func defineRule(name string, category string, defaultOn bool, severity string, description string, help string, fn func(context.Context, *analysisContext, Descriptor) ([]model.Diagnostic, []model.ToolError)) rule {
	desc := Descriptor{
		Plugin:      "repo",
		Rule:        name,
		Category:    category,
		DefaultOn:   defaultOn,
		Severity:    severity,
		Description: description,
		Help:        help,
	}
	return rule{
		desc: desc,
		run: func(ctx context.Context, pass *analysisContext) ([]model.Diagnostic, []model.ToolError) {
			return fn(ctx, pass, desc)
		},
	}
}

func loadAnalysisContext(target diagnostics.Target) (*analysisContext, []model.ToolError) {
	repoRoot, err := filepath.Abs(target.RepoRoot)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "repo-hygiene",
			Message: fmt.Sprintf("resolve repo root: %v", err),
			Fatal:   true,
		}}
	}

	moduleRoots := make([]string, 0, len(target.ModuleRoots))
	for _, root := range target.ModuleRoots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, []model.ToolError{{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("resolve module root %q: %v", root, err),
				Fatal:   true,
			}}
		}
		moduleRoots = append(moduleRoots, filepath.Clean(absRoot))
	}
	if len(target.ModulePatterns) > 0 {
		moduleRoots = filterModuleRoots(moduleRoots, target.ModulePatterns)
		if len(moduleRoots) == 0 {
			return nil, []model.ToolError{{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("no modules matched filter %q", strings.Join(target.ModulePatterns, ",")),
				Fatal:   true,
			}}
		}
	}

	modules := make([]moduleInfo, 0, len(moduleRoots))
	for _, root := range moduleRoots {
		relRoot, err := filepath.Rel(repoRoot, root)
		if err != nil {
			return nil, []model.ToolError{{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("resolve module path %q: %v", root, err),
				Fatal:   true,
			}}
		}
		modulePath, err := readModulePath(filepath.Join(root, "go.mod"))
		if err != nil {
			return nil, []model.ToolError{{
				Tool:    "repo-hygiene",
				Message: err.Error(),
				Fatal:   true,
			}}
		}
		modules = append(modules, moduleInfo{
			root:       root,
			relRoot:    model.NormalizePath(relRoot),
			modulePath: modulePath,
		})
	}

	workspaceRel := ""
	if _, err := os.Stat(filepath.Join(repoRoot, "go.work")); err == nil {
		workspaceRel = "go.work"
	}

	return &analysisContext{
		target:       target,
		repoRoot:     filepath.Clean(repoRoot),
		moduleRoots:  modules,
		workspaceRel: workspaceRel,
	}, nil
}

func runModNotTidy(ctx context.Context, pass *analysisContext, desc Descriptor) ([]model.Diagnostic, []model.ToolError) {
	var diagnosticsOut []model.Diagnostic
	var toolErrors []model.ToolError

	for _, module := range pass.moduleRoots {
		tempRoot, err := copyRepo(ctx, pass.repoRoot)
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: copy repo for %s: %v", desc.Rule, moduleDisplayPath(module), err),
			})
			continue
		}

		tempModuleRoot := filepath.Join(tempRoot, filepath.FromSlash(module.relRoot))
		result, err := runCommand(ctx, tempModuleRoot, "go", "mod", "tidy")
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: run go mod tidy for %s: %v", desc.Rule, moduleDisplayPath(module), missingToolError("go", "install Go and ensure go is on PATH", err)),
			})
			if removeErr := os.RemoveAll(tempRoot); removeErr != nil {
				toolErrors = append(toolErrors, model.ToolError{
					Tool:    "repo-hygiene",
					Message: fmt.Sprintf("%s: remove temp copy for %s: %v", desc.Rule, moduleDisplayPath(module), removeErr),
				})
			}
			continue
		}
		if result.exitCode != 0 {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: go mod tidy failed for %s: %s", desc.Rule, moduleDisplayPath(module), combinedOutput(result)),
			})
			if removeErr := os.RemoveAll(tempRoot); removeErr != nil {
				toolErrors = append(toolErrors, model.ToolError{
					Tool:    "repo-hygiene",
					Message: fmt.Sprintf("%s: remove temp copy for %s: %v", desc.Rule, moduleDisplayPath(module), removeErr),
				})
			}
			continue
		}

		for _, name := range []string{"go.mod", "go.sum"} {
			changed, beforeExists, afterExists, err := filesDiffer(
				filepath.Join(module.root, name),
				filepath.Join(tempModuleRoot, name),
			)
			if err != nil {
				toolErrors = append(toolErrors, model.ToolError{
					Tool:    "repo-hygiene",
					Message: fmt.Sprintf("%s: compare %s for %s: %v", desc.Rule, name, moduleDisplayPath(module), err),
				})
				continue
			}
			if !changed {
				continue
			}

			relPath := model.NormalizePath(filepath.Join(module.relRoot, name))
			if module.relRoot == "." {
				relPath = name
			}
			diagnosticsOut = append(diagnosticsOut, fileDiagnostic(desc, relPath, 1, 1, tidyMessage(name, beforeExists, afterExists), module.modulePath))
		}
		if removeErr := os.RemoveAll(tempRoot); removeErr != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: remove temp copy for %s: %v", desc.Rule, moduleDisplayPath(module), removeErr),
			})
		}
	}

	return diagnosticsOut, toolErrors
}

func runReplaceLocalPath(_ context.Context, pass *analysisContext, desc Descriptor) ([]model.Diagnostic, []model.ToolError) {
	var diagnosticsOut []model.Diagnostic
	var toolErrors []model.ToolError

	for _, module := range pass.moduleRoots {
		filePath := filepath.Join(module.root, "go.mod")
		raw, err := os.ReadFile(filePath)
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: read %s: %v", desc.Rule, filePath, err),
			})
			continue
		}

		parsed, err := modfile.Parse(filePath, raw, nil)
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: parse %s: %v", desc.Rule, filePath, err),
			})
			continue
		}

		relPath := "go.mod"
		if module.relRoot != "." {
			relPath = model.NormalizePath(filepath.Join(module.relRoot, "go.mod"))
		}
		for _, replace := range parsed.Replace {
			if !isLocalReplacePath(replace.New.Path, replace.New.Version) {
				continue
			}
			line, column := lineColumnFromReplace(replace)
			diagnosticsOut = append(diagnosticsOut, fileDiagnostic(
				desc,
				relPath,
				line,
				column,
				fmt.Sprintf("local replace target %q for %s reduces reproducibility", replace.New.Path, replace.Old.Path),
				module.modulePath,
			))
		}
	}

	if pass.workspaceRel != "" {
		workspacePath := filepath.Join(pass.repoRoot, pass.workspaceRel)
		raw, err := os.ReadFile(workspacePath)
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: read %s: %v", desc.Rule, workspacePath, err),
			})
			return diagnosticsOut, toolErrors
		}
		parsed, err := modfile.ParseWork(workspacePath, raw, nil)
		if err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: parse %s: %v", desc.Rule, workspacePath, err),
			})
			return diagnosticsOut, toolErrors
		}
		for _, replace := range parsed.Replace {
			if !isLocalReplacePath(replace.New.Path, replace.New.Version) {
				continue
			}
			line, column := lineColumnFromReplace(replace)
			diagnosticsOut = append(diagnosticsOut, fileDiagnostic(
				desc,
				pass.workspaceRel,
				line,
				column,
				fmt.Sprintf("local replace target %q for %s reduces reproducibility", replace.New.Path, replace.Old.Path),
				"",
			))
		}
	}

	return diagnosticsOut, toolErrors
}

func runModReadonlyFailure(ctx context.Context, pass *analysisContext, desc Descriptor) ([]model.Diagnostic, []model.ToolError) {
	tempRoot, err := copyRepo(ctx, pass.repoRoot)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "repo-hygiene",
			Message: fmt.Sprintf("%s: copy repo: %v", desc.Rule, err),
		}}
	}
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	commandDir := tempRoot
	if pass.workspaceRel == "" && len(pass.moduleRoots) == 1 {
		commandDir = filepath.Join(tempRoot, filepath.FromSlash(pass.moduleRoots[0].relRoot))
	}

	args := append([]string{"list", "-deps", "-mod=readonly"}, packagePatterns(pass.target, pass.moduleRoots)...)
	result, err := runCommand(ctx, commandDir, "go", args...)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "repo-hygiene",
			Message: fmt.Sprintf("%s: run go %s: %v", desc.Rule, strings.Join(args, " "), missingToolError("go", "install Go and ensure go is on PATH", err)),
		}}
	}
	if result.exitCode == 0 {
		return nil, nil
	}

	output := combinedOutput(result)
	if !looksLikeReadonlyFailure(output) {
		return nil, []model.ToolError{{
			Tool:    "repo-hygiene",
			Message: fmt.Sprintf("%s: go %s failed: %s", desc.Rule, strings.Join(args, " "), output),
		}}
	}

	pathname, modulePath := readonlyAnchor(pass)
	message := "go list -deps -mod=readonly failed because dependency metadata is incomplete"
	if summary := readonlySummary(output); summary != "" {
		message = summary
	}

	return []model.Diagnostic{
		fileDiagnostic(desc, pathname, 1, 1, message, modulePath),
	}, nil
}

func runNotGofmt(ctx context.Context, pass *analysisContext, desc Descriptor) ([]model.Diagnostic, []model.ToolError) {
	files, err := goFiles(pass)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "repo-hygiene",
			Message: fmt.Sprintf("%s: scan Go files: %v", desc.Rule, err),
		}}
	}
	if len(files) == 0 {
		return nil, nil
	}

	var diagnosticsOut []model.Diagnostic
	var toolErrors []model.ToolError
	for _, chunk := range chunkFiles(files, 64) {
		args := append([]string{"-l"}, chunk...)
		result, err := runCommand(ctx, pass.repoRoot, "gofmt", args...)
		if err != nil {
			return nil, []model.ToolError{{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: run gofmt: %v", desc.Rule, missingToolError("gofmt", "reinstall Go and ensure gofmt is on PATH", err)),
			}}
		}
		if result.exitCode != 0 {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: gofmt failed: %s", desc.Rule, combinedOutput(result)),
			})
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(result.stdout))
		for scanner.Scan() {
			pathname := model.NormalizePath(strings.TrimSpace(scanner.Text()))
			if pathname == "" {
				continue
			}
			modulePath := modulePathForFile(pass.moduleRoots, pathname)
			diagnosticsOut = append(diagnosticsOut, fileDiagnostic(desc, pathname, 1, 1, "file is not gofmt formatted", modulePath))
		}
		if err := scanner.Err(); err != nil {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "repo-hygiene",
				Message: fmt.Sprintf("%s: read gofmt output: %v", desc.Rule, err),
			})
		}
	}

	return diagnosticsOut, toolErrors
}

func runMissingLicense(_ context.Context, pass *analysisContext, desc Descriptor) ([]model.Diagnostic, []model.ToolError) {
	if hasLicenseFile(pass.repoRoot) {
		return nil, nil
	}

	pathname := "go.mod"
	modulePath := ""
	if pass.workspaceRel != "" {
		pathname = pass.workspaceRel
	} else if len(pass.moduleRoots) > 0 {
		modulePath = pass.moduleRoots[0].modulePath
		if pass.moduleRoots[0].relRoot != "." {
			pathname = model.NormalizePath(filepath.Join(pass.moduleRoots[0].relRoot, "go.mod"))
		}
	}

	return []model.Diagnostic{
		fileDiagnostic(desc, pathname, 1, 1, "repository is missing a license file", modulePath),
	}, nil
}

func readModulePath(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	parsed, err := modfile.Parse(path, raw, nil)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", path, err)
	}
	if parsed.Module == nil {
		return "", nil
	}
	return parsed.Module.Mod.Path, nil
}

func fileDiagnostic(desc Descriptor, path string, line int, column int, message string, modulePath string) model.Diagnostic {
	return model.Diagnostic{
		Path:      model.NormalizePath(path),
		Line:      max(line, 1),
		Column:    max(column, 1),
		EndLine:   max(line, 1),
		EndColumn: max(column, 1),
		Plugin:    desc.Plugin,
		Rule:      desc.Rule,
		Severity:  desc.Severity,
		Category:  desc.Category,
		Message:   message,
		Help:      desc.Help,
		Module:    modulePath,
	}
}

func tidyMessage(name string, beforeExists bool, afterExists bool) string {
	switch {
	case !beforeExists && afterExists:
		return fmt.Sprintf("go mod tidy would create %s", name)
	case beforeExists && !afterExists:
		return fmt.Sprintf("go mod tidy would remove %s", name)
	default:
		return fmt.Sprintf("go mod tidy would update %s", name)
	}
}

func readonlyAnchor(pass *analysisContext) (string, string) {
	if pass.workspaceRel != "" {
		return pass.workspaceRel, ""
	}
	if len(pass.moduleRoots) == 0 {
		return "go.mod", ""
	}
	pathname := "go.mod"
	if pass.moduleRoots[0].relRoot != "." {
		pathname = model.NormalizePath(filepath.Join(pass.moduleRoots[0].relRoot, "go.mod"))
	}
	return pathname, pass.moduleRoots[0].modulePath
}

func readonlySummary(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.Contains(line, "missing go.sum entry"):
			return "go list -mod=readonly failed because go.sum is missing required entries"
		case strings.Contains(line, "updates to go.mod needed"):
			return "go list -mod=readonly failed because go.mod needs updates"
		case strings.Contains(line, "updates to go.work needed"):
			return "go list -mod=readonly failed because go.work needs updates"
		}
	}
	return ""
}

func looksLikeReadonlyFailure(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "missing go.sum entry") ||
		strings.Contains(lower, "updates to go.mod needed") ||
		strings.Contains(lower, "updates to go.work needed") ||
		strings.Contains(lower, "updates to go.sum needed")
}

func lineColumnFromReplace(replace *modfile.Replace) (int, int) {
	if replace == nil || replace.Syntax == nil {
		return 1, 1
	}
	return max(replace.Syntax.Start.Line, 1), max(replace.Syntax.Start.LineRune, 1)
}

func isLocalReplacePath(path string, version string) bool {
	if version != "" {
		return false
	}
	if filepath.IsAbs(path) {
		return true
	}
	path = filepath.ToSlash(path)
	return path == "." || path == ".." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func hasLicenseFile(repoRoot string) bool {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToUpper(entry.Name()) {
		case "LICENSE", "LICENSE.TXT", "LICENSE.MD", "COPYING", "COPYING.TXT", "COPYRIGHT", "UNLICENSE":
			return true
		}
	}
	return false
}

func goFiles(pass *analysisContext) ([]string, error) {
	var files []string
	for _, module := range pass.moduleRoots {
		err := filepath.WalkDir(module.root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != module.root {
					name := entry.Name()
					if name == ".git" || name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || isNestedModule(path, module.root) {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			relDir, err := filepath.Rel(pass.repoRoot, filepath.Dir(path))
			if err != nil {
				return err
			}
			if !matchesPackagePatterns(model.NormalizePath(relDir), pass.target.PackagePatterns) {
				return nil
			}
			if !pass.target.IncludeGenerated {
				generated, err := isGeneratedGoFile(path)
				if err != nil {
					return err
				}
				if generated {
					return nil
				}
			}
			relPath, err := filepath.Rel(pass.repoRoot, path)
			if err != nil {
				return err
			}
			files = append(files, model.NormalizePath(relPath))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

func modulePathForFile(modules []moduleInfo, relPath string) string {
	relPath = model.NormalizePath(relPath)
	for _, module := range modules {
		prefix := module.relRoot
		if prefix == "." {
			return module.modulePath
		}
		if relPath == prefix || strings.HasPrefix(relPath, prefix+"/") {
			return module.modulePath
		}
	}
	return ""
}

func matchesPackagePatterns(relDir string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	relDir = model.NormalizePath(relDir)
	for _, pattern := range patterns {
		normalized := model.NormalizePath(strings.TrimPrefix(pattern, "./"))
		switch {
		case pattern == "./..." || pattern == "..." || normalized == "." || normalized == "":
			return true
		case strings.HasSuffix(normalized, "/..."):
			prefix := strings.TrimSuffix(normalized, "/...")
			if prefix == "" || prefix == "." || relDir == prefix || strings.HasPrefix(relDir, prefix+"/") {
				return true
			}
		case relDir == normalized:
			return true
		}
	}
	return false
}

func packagePatterns(target diagnostics.Target, modules []moduleInfo) []string {
	if len(target.PackagePatterns) > 0 {
		return append([]string(nil), target.PackagePatterns...)
	}
	if len(target.ModulePatterns) > 0 {
		patterns := make([]string, 0, len(modules))
		for _, module := range modules {
			if module.relRoot == "." {
				patterns = append(patterns, "./...")
				continue
			}
			patterns = append(patterns, "./"+module.relRoot+"/...")
		}
		if len(patterns) > 0 {
			return patterns
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

func copyRepo(ctx context.Context, root string) (string, error) {
	tempRoot, err := os.MkdirTemp("", "go-doctor-repo-*")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tempRoot)
		}
	}()

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if entry.IsDir() {
			if shouldSkipCopyDir(entry.Name()) {
				return filepath.SkipDir
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(filepath.Join(tempRoot, relPath), info.Mode())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, filepath.Join(tempRoot, relPath))
		}
		return copyFile(path, filepath.Join(tempRoot, relPath), entry)
	})
	if err != nil {
		return "", err
	}

	success = true
	return tempRoot, nil
}

func copyFile(source string, dest string, entry os.DirEntry) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}

	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func shouldSkipCopyDir(name string) bool {
	return name == ".git" || name == "vendor"
}

func filesDiffer(left string, right string) (changed bool, leftExists bool, rightExists bool, err error) {
	leftBytes, leftExists, err := readOptionalFile(left)
	if err != nil {
		return false, false, false, err
	}
	rightBytes, rightExists, err := readOptionalFile(right)
	if err != nil {
		return false, false, false, err
	}
	return !bytes.Equal(leftBytes, rightBytes) || leftExists != rightExists, leftExists, rightExists, nil
}

func readOptionalFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func runCommand(ctx context.Context, dir string, name string, args ...string) (execResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := execResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func missingToolError(tool string, installHint string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%s is not installed or not on PATH; install with: %s", tool, installHint)
	}
	return err
}

func combinedOutput(result execResult) string {
	stdout := strings.TrimSpace(result.stdout)
	stderr := strings.TrimSpace(result.stderr)
	switch {
	case stdout != "" && stderr != "":
		return stdout + "\n" + stderr
	case stdout != "":
		return stdout
	default:
		return stderr
	}
}

func chunkFiles(files []string, size int) [][]string {
	if len(files) == 0 {
		return nil
	}
	if size < 1 {
		size = len(files)
	}
	var chunks [][]string
	for len(files) > 0 {
		n := size
		if n > len(files) {
			n = len(files)
		}
		chunks = append(chunks, append([]string(nil), files[:n]...))
		files = files[n:]
	}
	return chunks
}

func isGeneratedGoFile(path string) (bool, error) {
	fileNode, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		return false, err
	}
	return ast.IsGenerated(fileNode), nil
}

func isNestedModule(path string, moduleRoot string) bool {
	if filepath.Clean(path) == filepath.Clean(moduleRoot) {
		return false
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

func moduleDisplayPath(module moduleInfo) string {
	if module.relRoot == "." {
		return "root module"
	}
	return module.relRoot
}

func max(value int, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}
