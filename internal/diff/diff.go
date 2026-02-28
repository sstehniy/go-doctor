package diff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

const (
	AutoBase = "auto"
)

type Options struct {
	RepoRoot    string
	ModuleRoots []string
	Base        string
}

type Plan struct {
	Narrowed        bool
	BaseCommit      string
	ChangedFiles    []ChangedFile
	IncludeFiles    []string
	PackagePatterns []string
	ModulePatterns  []string
	Warnings        []string
}

type ChangedFile struct {
	Path    string
	Deleted bool
}

func Discover(ctx context.Context, opts Options) (Plan, error) {
	root, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve repo root: %w", err)
	}
	moduleRoots, err := normalizeModuleRoots(root, opts.ModuleRoots)
	if err != nil {
		return Plan{}, err
	}

	if strings.TrimSpace(opts.Base) == "" {
		return Plan{Narrowed: false}, nil
	}

	plan := Plan{}
	base := strings.TrimSpace(opts.Base)
	if strings.EqualFold(base, AutoBase) {
		defaultRef, detectErr := detectRemoteDefaultBranch(ctx, root)
		if detectErr == nil && defaultRef != "" {
			baseCommit, mergeErr := mergeBase(ctx, root, defaultRef)
			if mergeErr == nil {
				plan.BaseCommit = baseCommit
				baseChanged, baseErr := changedFilesFromBase(ctx, root, baseCommit)
				if baseErr == nil {
					localChanged, localErr := changedFilesFromIndexAndWorktree(ctx, root)
					if localErr != nil {
						plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve staged/unstaged changes failed: %v", localErr))
					}
					plan.ChangedFiles = dedupeChangedFiles(append(baseChanged, localChanged...))
					plan.Narrowed = true
					finalizePlan(&plan, root, moduleRoots)
					return plan, nil
				}
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve diff from %s failed: %v", defaultRef, baseErr))
			} else {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve merge-base with %s failed: %v", defaultRef, mergeErr))
			}
		}

		stagedUnstaged, stagedErr := changedFilesFromIndexAndWorktree(ctx, root)
		if stagedErr != nil {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve staged/unstaged changes failed: %v", stagedErr))
			return plan, nil
		}
		if len(stagedUnstaged) == 0 {
			plan.Warnings = append(plan.Warnings, "diff mode found no remote default branch and no staged/unstaged changes; running full scan")
			return plan, nil
		}
		plan.Narrowed = true
		plan.ChangedFiles = stagedUnstaged
		finalizePlan(&plan, root, moduleRoots)
		return plan, nil
	}

	baseCommit, mergeErr := mergeBase(ctx, root, base)
	if mergeErr != nil {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve merge-base with %s failed: %v", base, mergeErr))
		return plan, nil
	}
	changed, changedErr := changedFilesFromBase(ctx, root, baseCommit)
	if changedErr != nil {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("resolve diff from %s failed: %v", base, changedErr))
		return plan, nil
	}
	plan.Narrowed = true
	plan.BaseCommit = baseCommit
	plan.ChangedFiles = changed
	finalizePlan(&plan, root, moduleRoots)
	return plan, nil
}

func normalizeModuleRoots(repoRoot string, roots []string) ([]string, error) {
	if len(roots) == 0 {
		return []string{repoRoot}, nil
	}
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve module root %q: %w", root, err)
		}
		normalized = append(normalized, filepath.Clean(absRoot))
	}
	sort.Strings(normalized)
	return normalized, nil
}

func detectRemoteDefaultBranch(ctx context.Context, repoRoot string) (string, error) {
	ref, err := git(ctx, repoRoot, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil && strings.TrimSpace(ref) != "" {
		return strings.TrimSpace(ref), nil
	}

	show, showErr := git(ctx, repoRoot, "remote", "show", "origin")
	if showErr != nil {
		if err != nil {
			return "", err
		}
		return "", showErr
	}
	for _, line := range strings.Split(show, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "HEAD branch:") {
			continue
		}
		branch := strings.TrimSpace(strings.TrimPrefix(trimmed, "HEAD branch:"))
		if branch == "" || branch == "(unknown)" {
			continue
		}
		return "origin/" + branch, nil
	}
	if err != nil {
		return "", err
	}
	return "", errors.New("origin default branch not found")
}

func mergeBase(ctx context.Context, repoRoot string, base string) (string, error) {
	output, err := git(ctx, repoRoot, "merge-base", "HEAD", base)
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(output)
	if hash == "" {
		return "", fmt.Errorf("empty merge-base for %s", base)
	}
	return hash, nil
}

func changedFilesFromBase(ctx context.Context, repoRoot string, baseCommit string) ([]ChangedFile, error) {
	output, err := git(ctx, repoRoot, "diff", "--name-status", "--no-renames", baseCommit+"...HEAD")
	if err != nil {
		return nil, err
	}
	return parseNameStatus(output), nil
}

func changedFilesFromIndexAndWorktree(ctx context.Context, repoRoot string) ([]ChangedFile, error) {
	cached, err := git(ctx, repoRoot, "diff", "--name-status", "--no-renames", "--cached")
	if err != nil {
		return nil, err
	}
	worktree, err := git(ctx, repoRoot, "diff", "--name-status", "--no-renames")
	if err != nil {
		return nil, err
	}
	untracked, err := git(ctx, repoRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	all := append(parseNameStatus(cached), parseNameStatus(worktree)...)
	all = append(all, parseNameList(untracked)...)
	return dedupeChangedFiles(all), nil
}

func parseNameList(output string) []ChangedFile {
	var out []ChangedFile
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		out = append(out, ChangedFile{
			Path:    model.NormalizePath(line),
			Deleted: false,
		})
	}
	return out
}

func parseNameStatus(output string) []ChangedFile {
	var out []ChangedFile
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		status := fields[0]
		switch {
		case strings.HasPrefix(status, "R") && len(fields) >= 3:
			out = append(out,
				ChangedFile{Path: model.NormalizePath(fields[1]), Deleted: true},
				ChangedFile{Path: model.NormalizePath(fields[2]), Deleted: false},
			)
		case strings.HasPrefix(status, "C") && len(fields) >= 3:
			out = append(out, ChangedFile{Path: model.NormalizePath(fields[2]), Deleted: false})
		case strings.HasPrefix(status, "D"):
			out = append(out, ChangedFile{Path: model.NormalizePath(fields[1]), Deleted: true})
		default:
			out = append(out, ChangedFile{Path: model.NormalizePath(fields[len(fields)-1]), Deleted: false})
		}
	}
	return dedupeChangedFiles(out)
}

func dedupeChangedFiles(changes []ChangedFile) []ChangedFile {
	type state struct {
		deleted bool
	}
	byPath := map[string]state{}
	for _, changed := range changes {
		if strings.TrimSpace(changed.Path) == "" {
			continue
		}
		existing, ok := byPath[changed.Path]
		if !ok {
			byPath[changed.Path] = state{deleted: changed.Deleted}
			continue
		}
		byPath[changed.Path] = state{deleted: existing.deleted && changed.Deleted}
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]ChangedFile, 0, len(paths))
	for _, path := range paths {
		out = append(out, ChangedFile{Path: path, Deleted: byPath[path].deleted})
	}
	return out
}

func finalizePlan(plan *Plan, repoRoot string, moduleRoots []string) {
	plan.IncludeFiles = includeFiles(plan.ChangedFiles)
	plan.PackagePatterns = packagePatterns(repoRoot, moduleRoots, plan.ChangedFiles)
	plan.ModulePatterns = modulePatterns(repoRoot, moduleRoots, plan.ChangedFiles)
}

func includeFiles(changes []ChangedFile) []string {
	var files []string
	for _, changed := range changes {
		if changed.Deleted {
			continue
		}
		files = append(files, changed.Path)
	}
	sort.Strings(files)
	return files
}

func packagePatterns(repoRoot string, moduleRoots []string, changes []ChangedFile) []string {
	set := map[string]struct{}{}
	for _, changed := range changes {
		if !affectsGoAnalysis(changed.Path) {
			continue
		}

		base := filepath.Base(changed.Path)
		switch {
		case base == "go.work":
			set["./..."] = struct{}{}
		case base == "go.mod" || base == "go.sum":
			moduleRoot := moduleRootForPath(repoRoot, moduleRoots, changed.Path)
			set[recursivePattern(repoRoot, moduleRoot)] = struct{}{}
		case strings.HasSuffix(changed.Path, ".go"):
			relDir := model.NormalizePath(filepath.Dir(changed.Path))
			if relDir == "." {
				set["./..."] = struct{}{}
			} else {
				set["./"+relDir+"/..."] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for pattern := range set {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func modulePatterns(repoRoot string, moduleRoots []string, changes []ChangedFile) []string {
	set := map[string]struct{}{}
	for _, changed := range changes {
		if !affectsGoAnalysis(changed.Path) {
			continue
		}
		if filepath.Base(changed.Path) == "go.work" {
			for _, root := range moduleRoots {
				set[model.NormalizePath(root)] = struct{}{}
			}
			continue
		}
		root := moduleRootForPath(repoRoot, moduleRoots, changed.Path)
		set[model.NormalizePath(root)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for pattern := range set {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func affectsGoAnalysis(path string) bool {
	if strings.HasSuffix(path, ".go") {
		return true
	}
	base := filepath.Base(path)
	return base == "go.mod" || base == "go.sum" || base == "go.work"
}

func recursivePattern(repoRoot string, moduleRoot string) string {
	rel, err := filepath.Rel(repoRoot, moduleRoot)
	if err != nil {
		return "./..."
	}
	rel = model.NormalizePath(rel)
	if rel == "." {
		return "./..."
	}
	return "./" + rel + "/..."
}

func moduleRootForPath(repoRoot string, moduleRoots []string, relPath string) string {
	absPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
	best := repoRoot
	bestLen := len(best)
	for _, root := range moduleRoots {
		root = filepath.Clean(root)
		if absPath == root || strings.HasPrefix(absPath, root+string(filepath.Separator)) {
			if len(root) > bestLen {
				best = root
				bestLen = len(root)
			}
		}
	}
	return best
}

func git(ctx context.Context, repoRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
