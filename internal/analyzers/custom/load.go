package custom

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type analysisContext struct {
	target            diagnostics.Target
	fset              *token.FileSet
	moduleInfos       []moduleInfo
	directories       []*directoryInfo
	dirByAbs          map[string]*directoryInfo
	fileByAbs         map[string]*sourceFile
	selectedDirs      map[string]struct{}
	typedPackages     []*typedPackage
	repoImportPathSet map[string]struct{}
}

type moduleInfo struct {
	root string
	path string
}

type directoryInfo struct {
	absDir     string
	relDir     string
	moduleRoot string
	modulePath string
	importPath string
	files      []*sourceFile
	nonTest    []*sourceFile
	tests      []*sourceFile
}

type sourceFile struct {
	absPath     string
	relPath     string
	relDir      string
	moduleRoot  string
	modulePath  string
	importPath  string
	packageName string
	isTest      bool
	generated   bool
	lineCount   int
	file        *ast.File
}

type typedPackage struct {
	pkg   *packages.Package
	dir   *directoryInfo
	files []*typedFile
}

type typedFile struct {
	source *sourceFile
	file   *ast.File
}

func loadAnalysisContext(ctx context.Context, target diagnostics.Target) (*analysisContext, []model.ToolError) {
	absRoot, err := filepath.Abs(target.RepoRoot)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "custom",
			Message: fmt.Sprintf("resolve repo root: %v", err),
			Fatal:   true,
		}}
	}
	target.RepoRoot = filepath.Clean(absRoot)
	for index, root := range target.ModuleRoots {
		absModuleRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, []model.ToolError{{
				Tool:    "custom",
				Message: fmt.Sprintf("resolve module root %q: %v", root, err),
				Fatal:   true,
			}}
		}
		target.ModuleRoots[index] = filepath.Clean(absModuleRoot)
	}

	moduleInfos, err := selectedModuleInfos(target)
	if err != nil {
		return nil, []model.ToolError{{
			Tool:    "custom",
			Message: err.Error(),
			Fatal:   true,
		}}
	}

	pass := &analysisContext{
		target:            target,
		fset:              token.NewFileSet(),
		moduleInfos:       moduleInfos,
		dirByAbs:          map[string]*directoryInfo{},
		fileByAbs:         map[string]*sourceFile{},
		selectedDirs:      map[string]struct{}{},
		repoImportPathSet: map[string]struct{}{},
	}

	if err := pass.scanSource(); err != nil {
		return nil, []model.ToolError{{
			Tool:    "custom",
			Message: err.Error(),
			Fatal:   true,
		}}
	}

	toolErrors := pass.loadTypedPackages(ctx)
	return pass, toolErrors
}

func (c *analysisContext) scanSource() error {
	buildContext := activeBuildContext()
	for _, moduleInfo := range c.moduleInfos {
		err := filepath.WalkDir(moduleInfo.root, func(pathname string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if pathname != moduleInfo.root {
					name := entry.Name()
					if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
						return filepath.SkipDir
					}
					if isNestedModule(pathname, moduleInfo.root) {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if filepath.Ext(pathname) != ".go" {
				return nil
			}
			matched, err := buildContext.MatchFile(filepath.Dir(pathname), filepath.Base(pathname))
			if err != nil {
				return fmt.Errorf("match build constraints for %s: %w", pathname, err)
			}
			if !matched {
				return nil
			}

			fileNode, err := parser.ParseFile(c.fset, pathname, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("parse %s: %w", pathname, err)
			}
			generated := ast.IsGenerated(fileNode)
			if generated && !c.target.IncludeGenerated {
				return nil
			}

			relPath, err := filepath.Rel(c.target.RepoRoot, pathname)
			if err != nil {
				return fmt.Errorf("resolve %s: %w", pathname, err)
			}
			relDir, err := filepath.Rel(c.target.RepoRoot, filepath.Dir(pathname))
			if err != nil {
				return fmt.Errorf("resolve dir %s: %w", pathname, err)
			}

			importPath := moduleInfo.path
			moduleRelative, err := filepath.Rel(moduleInfo.root, filepath.Dir(pathname))
			if err != nil {
				return fmt.Errorf("resolve module dir %s: %w", pathname, err)
			}
			moduleRelative = model.NormalizePath(moduleRelative)
			if moduleRelative != "." {
				importPath = path.Join(moduleInfo.path, moduleRelative)
			}

			lineCount, err := fileLineCount(pathname)
			if err != nil {
				return err
			}

			source := &sourceFile{
				absPath:     filepath.Clean(pathname),
				relPath:     model.NormalizePath(relPath),
				relDir:      model.NormalizePath(relDir),
				moduleRoot:  moduleInfo.root,
				modulePath:  moduleInfo.path,
				importPath:  importPath,
				packageName: fileNode.Name.Name,
				isTest:      strings.HasSuffix(pathname, "_test.go"),
				generated:   generated,
				lineCount:   lineCount,
				file:        fileNode,
			}
			c.fileByAbs[source.absPath] = source

			dir := c.ensureDirectory(filepath.Dir(pathname), source.relDir, moduleInfo)
			dir.files = append(dir.files, source)
			if source.isTest {
				dir.tests = append(dir.tests, source)
			} else {
				dir.nonTest = append(dir.nonTest, source)
			}
			c.repoImportPathSet[source.importPath] = struct{}{}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *analysisContext) ensureDirectory(absDir string, relDir string, moduleInfo moduleInfo) *directoryInfo {
	absDir = filepath.Clean(absDir)
	if existing, ok := c.dirByAbs[absDir]; ok {
		return existing
	}
	importPath := moduleInfo.path
	moduleRelative, err := filepath.Rel(moduleInfo.root, absDir)
	if err == nil {
		moduleRelative = model.NormalizePath(moduleRelative)
		if moduleRelative != "." {
			importPath = path.Join(moduleInfo.path, moduleRelative)
		}
	}
	dir := &directoryInfo{
		absDir:     absDir,
		relDir:     model.NormalizePath(relDir),
		moduleRoot: moduleInfo.root,
		modulePath: moduleInfo.path,
		importPath: importPath,
	}
	c.dirByAbs[absDir] = dir
	c.directories = append(c.directories, dir)
	return dir
}

func (c *analysisContext) loadTypedPackages(ctx context.Context) []model.ToolError {
	config := &packages.Config{
		Context: ctx,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedModule,
		Dir:  c.target.RepoRoot,
		Fset: c.fset,
	}

	pkgs, err := packages.Load(config, packagePatterns(c.target)...)
	if err != nil {
		return []model.ToolError{{
			Tool:    "custom",
			Message: fmt.Sprintf("load packages: %v", err),
		}}
	}

	var toolErrors []model.ToolError
	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			toolErrors = append(toolErrors, model.ToolError{
				Tool:    "custom",
				Message: pkgErr.Error(),
			})
		}

		typed := &typedPackage{pkg: pkg}
		for index, pathname := range pkg.CompiledGoFiles {
			source := c.fileByAbs[filepath.Clean(pathname)]
			if source == nil {
				continue
			}
			if source.generated && !c.target.IncludeGenerated {
				continue
			}
			if typed.dir == nil {
				typed.dir = c.dirByAbs[filepath.Clean(filepath.Dir(pathname))]
			}
			typed.files = append(typed.files, &typedFile{
				source: source,
				file:   pkg.Syntax[index],
			})
			c.selectedDirs[source.relDir] = struct{}{}
		}
		if len(typed.files) == 0 {
			continue
		}
		if typed.dir != nil {
			typed.dir.importPath = pkg.PkgPath
			for _, file := range typed.dir.files {
				file.importPath = pkg.PkgPath
			}
		}
		c.typedPackages = append(c.typedPackages, typed)
		c.repoImportPathSet[pkg.PkgPath] = struct{}{}
	}

	if len(c.selectedDirs) == 0 && len(c.target.PackagePatterns) == 0 {
		for _, dir := range c.directories {
			c.selectedDirs[dir.relDir] = struct{}{}
		}
	}

	return toolErrors
}

func selectedModuleInfos(target diagnostics.Target) ([]moduleInfo, error) {
	roots := target.ModuleRoots
	if len(target.ModulePatterns) > 0 {
		roots = filterModuleRoots(target.ModuleRoots, target.ModulePatterns)
		if len(roots) == 0 {
			return nil, fmt.Errorf("no modules matched filter %q", strings.Join(target.ModulePatterns, ","))
		}
	}
	if len(roots) == 0 {
		roots = []string{target.RepoRoot}
	}

	out := make([]moduleInfo, 0, len(roots))
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve module root %q: %w", root, err)
		}
		modulePath, err := parseModulePath(filepath.Join(absRoot, "go.mod"))
		if err != nil {
			return nil, err
		}
		out = append(out, moduleInfo{
			root: filepath.Clean(absRoot),
			path: modulePath,
		})
	}
	return out, nil
}

func parseModulePath(pathname string) (string, error) {
	file, err := os.Open(pathname)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", pathname, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %s: %w", pathname, err)
	}
	return "", fmt.Errorf("module path missing in %s", pathname)
}

func fileLineCount(pathname string) (int, error) {
	raw, err := os.ReadFile(pathname)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", pathname, err)
	}
	if len(raw) == 0 {
		return 0, nil
	}
	count := 1
	for _, value := range raw {
		if value == '\n' {
			count++
		}
	}
	return count, nil
}

func packagePatterns(target diagnostics.Target) []string {
	if len(target.PackagePatterns) > 0 {
		out := make([]string, len(target.PackagePatterns))
		copy(out, target.PackagePatterns)
		return out
	}
	if len(target.ModulePatterns) > 0 {
		moduleRoots := filterModuleRoots(target.ModuleRoots, target.ModulePatterns)
		if patterns := patternsForModuleRoots(target.RepoRoot, moduleRoots); len(patterns) > 0 {
			return patterns
		}
	}
	if patterns := patternsForModuleRoots(target.RepoRoot, target.ModuleRoots); len(patterns) > 0 {
		return patterns
	}
	return []string{"./..."}
}

func patternsForModuleRoots(repoRoot string, moduleRoots []string) []string {
	patterns := make([]string, 0, len(moduleRoots))
	for _, root := range moduleRoots {
		relative, err := filepath.Rel(repoRoot, root)
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
	return slices.Compact(patterns)
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

func unquoteImport(value string) string {
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return value
	}
	return unquoted
}

func isNestedModule(pathname string, moduleRoot string) bool {
	if filepath.Clean(pathname) == filepath.Clean(moduleRoot) {
		return false
	}
	if _, err := os.Stat(filepath.Join(pathname, "go.mod")); err == nil {
		return true
	}
	return false
}

func activeBuildContext() build.Context {
	ctx := build.Default
	ctx.BuildTags = append(ctx.BuildTags, goBuildTagsFromEnv(os.Getenv("GOFLAGS"))...)
	return ctx
}

func goBuildTagsFromEnv(goflags string) []string {
	fields := strings.Fields(goflags)
	var tags []string
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		value := ""
		switch {
		case strings.HasPrefix(field, "-tags="):
			value = strings.TrimPrefix(field, "-tags=")
		case strings.HasPrefix(field, "--tags="):
			value = strings.TrimPrefix(field, "--tags=")
		case field == "-tags" || field == "--tags":
			if index+1 < len(fields) {
				index++
				value = fields[index]
			}
		}
		for _, tag := range strings.Split(value, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tags = append(tags, tag)
		}
	}
	return slices.Compact(tags)
}
