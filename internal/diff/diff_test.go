package diff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sstehniy/go-doctor/internal/model"
)

func TestDiscoverExplicitBaseUsesMergeBaseAndDeletedFilesAffectPackages(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/test\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(repo, "a", "a.go"), "package a\nfunc A(){}\n")
	writeFile(t, filepath.Join(repo, "b", "deleted.go"), "package b\nfunc B(){}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	runGit(t, repo, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(repo, "a", "a.go"), "package a\nfunc A(){println(\"x\")}\n")
	if err := os.Remove(filepath.Join(repo, "b", "deleted.go")); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "change")

	plan, err := Discover(context.Background(), Options{
		RepoRoot:    repo,
		ModuleRoots: []string{repo},
		Base:        "main",
	})
	if err != nil {
		t.Fatalf("discover diff: %v", err)
	}
	if !plan.Narrowed {
		t.Fatalf("expected narrowed plan, got %#v", plan)
	}
	if plan.BaseCommit == "" {
		t.Fatalf("expected merge-base commit, got %#v", plan)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", plan.Warnings)
	}

	wantChanged := []ChangedFile{
		{Path: "a/a.go", Deleted: false},
		{Path: "b/deleted.go", Deleted: true},
	}
	if !slices.Equal(plan.ChangedFiles, wantChanged) {
		t.Fatalf("unexpected changed files\nwant: %#v\ngot: %#v", wantChanged, plan.ChangedFiles)
	}
	if !slices.Equal(plan.IncludeFiles, []string{"a/a.go"}) {
		t.Fatalf("unexpected include files: %#v", plan.IncludeFiles)
	}
	if !slices.Equal(plan.PackagePatterns, []string{"./a/...", "./b/..."}) {
		t.Fatalf("unexpected package patterns: %#v", plan.PackagePatterns)
	}
}

func TestDiscoverAutoUsesRemoteDefaultBranchWhenAvailable(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/test\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(repo, "a.go"), "package main\nfunc main(){}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	mainHead := runGit(t, repo, "rev-parse", "HEAD")

	runGit(t, repo, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(repo, "a.go"), "package main\nfunc main(){println(\"changed\")}\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "change")
	writeFile(t, filepath.Join(repo, "local_only.go"), "package main\nvar localOnly = 1\n")

	runGit(t, repo, "update-ref", "refs/remotes/origin/main", mainHead)
	runGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	plan, err := Discover(context.Background(), Options{
		RepoRoot:    repo,
		ModuleRoots: []string{repo},
		Base:        AutoBase,
	})
	if err != nil {
		t.Fatalf("discover diff: %v", err)
	}
	if !plan.Narrowed {
		t.Fatalf("expected narrowed plan, got %#v", plan)
	}
	if !slices.Equal(plan.IncludeFiles, []string{"a.go", "local_only.go"}) {
		t.Fatalf("unexpected include files: %#v", plan.IncludeFiles)
	}
}

func TestDiscoverAutoFallsBackToStagedAndUnstagedChanges(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/test\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(repo, "a.go"), "package main\nfunc main(){}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, filepath.Join(repo, "a.go"), "package main\nfunc main(){println(\"changed\")}\n")

	plan, err := Discover(context.Background(), Options{
		RepoRoot:    repo,
		ModuleRoots: []string{repo},
		Base:        AutoBase,
	})
	if err != nil {
		t.Fatalf("discover diff: %v", err)
	}
	if !plan.Narrowed {
		t.Fatalf("expected narrowed plan, got %#v", plan)
	}
	if plan.BaseCommit != "" {
		t.Fatalf("expected no base commit when using staged/unstaged fallback, got %q", plan.BaseCommit)
	}
	if !slices.Equal(plan.IncludeFiles, []string{"a.go"}) {
		t.Fatalf("unexpected include files: %#v", plan.IncludeFiles)
	}
}

func TestDiscoverAutoWarnsAndFallsBackToFullScanWhenNoDiffSource(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/test\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(repo, "a.go"), "package main\nfunc main(){}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	plan, err := Discover(context.Background(), Options{
		RepoRoot:    repo,
		ModuleRoots: []string{repo},
		Base:        AutoBase,
	})
	if err != nil {
		t.Fatalf("discover diff: %v", err)
	}
	if plan.Narrowed {
		t.Fatalf("expected full-scan fallback plan, got %#v", plan)
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected warning for full-scan fallback, got %#v", plan)
	}
}

func TestPackageAndModulePatternsRouteGoWorkChangesAcrossWorkspace(t *testing.T) {
	repoRoot := filepath.FromSlash("/repo")
	moduleRoots := []string{
		filepath.Join(repoRoot, "moda"),
		filepath.Join(repoRoot, "modb"),
	}
	changes := []ChangedFile{
		{Path: "go.work", Deleted: false},
	}

	gotPackages := packagePatterns(repoRoot, moduleRoots, changes)
	if !slices.Equal(gotPackages, []string{"./..."}) {
		t.Fatalf("unexpected package patterns: %#v", gotPackages)
	}

	gotModules := modulePatterns(repoRoot, moduleRoots, changes)
	wantModules := []string{
		model.NormalizePath(filepath.Join(repoRoot, "moda")),
		model.NormalizePath(filepath.Join(repoRoot, "modb")),
	}
	if !slices.Equal(gotModules, wantModules) {
		t.Fatalf("unexpected module patterns: %#v", gotModules)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	return repo
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out := runCommand(t, dir, "git", args...)
	return strings.TrimSpace(out)
}

func runCommand(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %s: %v (%s)", name, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
