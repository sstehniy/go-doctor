package repohygiene

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
)

func TestRepoHygieneRuleFixtures(t *testing.T) {
	testCases := []struct {
		name    string
		root    string
		enable  []string
		want    []string
		wantErr int
	}{
		{
			name:   "mod not tidy",
			root:   fixtureRoot("not-tidy"),
			enable: []string{"mod/not-tidy"},
			want:   []string{"mod/not-tidy:go.mod"},
		},
		{
			name:   "mod replace local path",
			root:   fixtureRoot("replace-local-path"),
			enable: []string{"mod/replace-local-path"},
			want:   []string{"mod/replace-local-path:go.mod"},
		},
		{
			name:   "build readonly failure",
			root:   fixtureRoot("mod-readonly-failure"),
			enable: []string{"build/mod-readonly-failure"},
			want:   []string{"build/mod-readonly-failure:go.mod"},
		},
		{
			name:    "broken import is not readonly drift",
			root:    fixtureRoot("broken-import"),
			enable:  []string{"build/mod-readonly-failure"},
			want:    nil,
			wantErr: 1,
		},
		{
			name:   "fmt not gofmt",
			root:   fixtureRoot("not-gofmt"),
			enable: []string{"fmt/not-gofmt"},
			want:   []string{"fmt/not-gofmt:main.go"},
		},
		{
			name:   "license missing",
			root:   fixtureRoot("missing-license"),
			enable: []string{"license/missing"},
			want:   []string{"license/missing:go.mod"},
		},
		{
			name:   "clean repo",
			root:   fixtureRoot("clean"),
			enable: []string{"repo"},
			want:   nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			target := diagnostics.Target{
				RepoRoot:    testCase.root,
				Mode:        "module",
				ModuleRoots: []string{testCase.root},
			}
			analyzers := DefaultAnalyzers(target, testCase.enable, nil)
			if len(analyzers) != 1 {
				t.Fatalf("expected one analyzer, got %d", len(analyzers))
			}

			result := analyzers[0].Run(context.Background(), target)
			if len(result.ToolErrors) != testCase.wantErr {
				t.Fatalf("expected %d tool errors, got %d: %#v", testCase.wantErr, len(result.ToolErrors), result.ToolErrors)
			}

			got := make([]string, 0, len(result.Diagnostics))
			for _, diagnostic := range result.Diagnostics {
				got = append(got, diagnostic.Rule+":"+diagnostic.Path)
			}
			slices.Sort(got)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("unexpected diagnostics\nwant: %v\ngot:  %v", testCase.want, got)
			}
		})
	}
}

func TestRepoHygieneDefaultOffRulesStayDisabled(t *testing.T) {
	selected, err := selectRules(nil, nil)
	if err != nil {
		t.Fatalf("select rules: %v", err)
	}

	selectedSet := map[string]struct{}{}
	for _, candidate := range selected {
		selectedSet[candidate.desc.Rule] = struct{}{}
	}

	for _, name := range []string{"fmt/not-gofmt", "license/missing"} {
		if _, ok := selectedSet[name]; ok {
			t.Fatalf("expected %s to stay default-off", name)
		}
	}
}

func TestRepoHygieneSelectionIgnoresOtherAnalyzerSelectors(t *testing.T) {
	target := diagnostics.Target{
		RepoRoot:    fixtureRoot("clean"),
		Mode:        "module",
		ModuleRoots: []string{fixtureRoot("clean")},
	}

	analyzers := DefaultAnalyzers(target, []string{"govet"}, nil)
	if len(analyzers) != 0 {
		t.Fatalf("expected no repo analyzer for third-party-only selection, got %d", len(analyzers))
	}

	analyzers = DefaultAnalyzers(target, []string{"govet", "mod/not-tidy"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one repo analyzer for mixed selection, got %d", len(analyzers))
	}
}

func TestModNotTidyDoesNotMutateWorkspace(t *testing.T) {
	root := fixtureRoot("not-tidy")
	goModPath := filepath.Join(root, "go.mod")
	before, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod before run: %v", err)
	}

	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}
	analyzers := DefaultAnalyzers(target, []string{"mod/not-tidy"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.ToolErrors) != 0 {
		t.Fatalf("expected no tool errors, got %#v", result.ToolErrors)
	}

	after, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod after run: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("expected mod/not-tidy to leave checked-out go.mod unchanged")
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Fatal("expected mod/not-tidy fixture to remain without go.sum in the checked-out workspace")
	}
}

func TestCopyRepoSkipsVendor(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/vendorcopy\n\ngo 1.22.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "vendor", "example.com", "dep"), 0o755); err != nil {
		t.Fatalf("mkdir vendor: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "example.com", "dep", "dep.go"), []byte("package dep\n"), 0o644); err != nil {
		t.Fatalf("write vendored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	tempRoot, err := copyRepo(context.Background(), root)
	if err != nil {
		t.Fatalf("copy repo: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	if _, err := os.Stat(filepath.Join(tempRoot, "vendor")); !os.IsNotExist(err) {
		t.Fatalf("expected vendor directory to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(tempRoot, "main.go")); err != nil {
		t.Fatalf("expected non-vendor files to be copied, got %v", err)
	}
}

func fixtureRoot(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "fixtures", "repo-hygiene", name)
}
