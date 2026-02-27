package discovery

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	Target       string
	Root         string
	Mode         string
	GoVersion    string
	ModuleRoots  []string
	PackageCount int
}

func Discover(target string) (Info, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return Info{}, fmt.Errorf("resolve target %q: %w", target, err)
	}
	stat, err := os.Stat(absTarget)
	if err != nil {
		return Info{}, fmt.Errorf("stat target %q: %w", target, err)
	}
	if !stat.IsDir() {
		absTarget = filepath.Dir(absTarget)
	}

	workRoot := findAncestorWith(absTarget, "go.work")
	if workRoot != "" {
		return discoverWorkspace(absTarget, workRoot)
	}

	modRoot := findAncestorWith(absTarget, "go.mod")
	if modRoot != "" {
		return discoverModule(absTarget, modRoot)
	}

	return Info{}, fmt.Errorf("no go.mod or go.work found from %s", normalize(absTarget))
}

func discoverModule(targetDir, moduleRoot string) (Info, error) {
	goVersion, err := parseGoVersion(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return Info{}, err
	}
	moduleRoots := []string{normalize(moduleRoot)}
	packageCount, err := countPackages([]string{moduleRoot})
	if err != nil {
		return Info{}, err
	}
	return Info{
		Target:       normalize(targetDir),
		Root:         normalize(moduleRoot),
		Mode:         "module",
		GoVersion:    goVersion,
		ModuleRoots:  moduleRoots,
		PackageCount: packageCount,
	}, nil
}

func discoverWorkspace(targetDir, workRoot string) (Info, error) {
	goVersion, moduleRoots, err := parseGoWork(filepath.Join(workRoot, "go.work"), workRoot)
	if err != nil {
		return Info{}, err
	}
	if goVersion == "" && len(moduleRoots) > 0 {
		goVersion, err = parseGoVersion(filepath.Join(moduleRoots[0], "go.mod"))
		if err != nil {
			return Info{}, err
		}
	}
	packageCount, err := countPackages(moduleRoots)
	if err != nil {
		return Info{}, err
	}
	normalizedRoots := make([]string, 0, len(moduleRoots))
	for _, root := range moduleRoots {
		normalizedRoots = append(normalizedRoots, normalize(root))
	}
	return Info{
		Target:       normalize(targetDir),
		Root:         normalize(workRoot),
		Mode:         "workspace",
		GoVersion:    goVersion,
		ModuleRoots:  normalizedRoots,
		PackageCount: packageCount,
	}, nil
}

func findAncestorWith(startDir, fileName string) string {
	current := startDir
	for {
		if _, err := os.Stat(filepath.Join(current, fileName)); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func parseGoVersion(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %q: %w", path, err)
	}
	return "", nil
}

func parseGoWork(path, workRoot string) (string, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	var (
		goVersion   string
		moduleRoots []string
		inUseBlock  bool
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "go "):
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		case line == "use (":
			inUseBlock = true
		case inUseBlock && line == ")":
			inUseBlock = false
		case strings.HasPrefix(line, "use "):
			moduleRoots = append(moduleRoots, resolveWorkUse(strings.TrimSpace(strings.TrimPrefix(line, "use ")), workRoot))
		case inUseBlock:
			moduleRoots = append(moduleRoots, resolveWorkUse(line, workRoot))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("scan %q: %w", path, err)
	}
	return goVersion, moduleRoots, nil
}

func resolveWorkUse(entry, workRoot string) string {
	entry = strings.Trim(entry, "\"")
	return filepath.Clean(filepath.Join(workRoot, filepath.FromSlash(entry)))
}

func countPackages(moduleRoots []string) (int, error) {
	packages := map[string]struct{}{}
	for _, root := range moduleRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			packages[filepath.Dir(path)] = struct{}{}
			return nil
		})
		if err != nil {
			return 0, fmt.Errorf("walk module %q: %w", root, err)
		}
	}
	return len(packages), nil
}

func normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
