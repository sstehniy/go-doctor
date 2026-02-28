package custom

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sstehniy/go-doctor/internal/diagnostics"
)

func TestCustomRuleFixtures(t *testing.T) {
	testCases := []struct {
		name string
		root string
		rule string
	}{
		{name: "error ignored return", root: fixtureRoot("error"), rule: "error/ignored-return"},
		{name: "error string compare", root: fixtureRoot("error"), rule: "error/string-compare"},
		{name: "error fmt wrap", root: fixtureRoot("error"), rule: "error/fmt-error-without-wrap"},
		{name: "context missing first arg", root: fixtureRoot("context"), rule: "context/missing-first-arg"},
		{name: "context background in request", root: fixtureRoot("context"), rule: "context/background-in-request-path"},
		{name: "context not propagated", root: fixtureRoot("context"), rule: "context/not-propagated"},
		{name: "context cancel", root: fixtureRoot("context"), rule: "context/with-timeout-not-canceled"},
		{name: "concurrency goroutine leak", root: fixtureRoot("concurrency"), rule: "concurrency/go-routine-leak-risk"},
		{name: "concurrency ticker stop", root: fixtureRoot("concurrency"), rule: "concurrency/ticker-not-stopped"},
		{name: "concurrency waitgroup misuse", root: fixtureRoot("concurrency"), rule: "concurrency/waitgroup-misuse"},
		{name: "concurrency mutex copy", root: fixtureRoot("concurrency"), rule: "concurrency/mutex-copy"},
		{name: "perf defer in loop", root: fixtureRoot("perf"), rule: "perf/defer-in-hot-loop"},
		{name: "perf fmt concat", root: fixtureRoot("perf"), rule: "perf/fmt-sprint-simple-concat"},
		{name: "perf bytes copy", root: fixtureRoot("perf"), rule: "perf/bytes-buffer-copy"},
		{name: "perf json unmarshal twice", root: fixtureRoot("perf"), rule: "perf/json-unmarshal-twice"},
		{name: "net request without context", root: fixtureRoot("net"), rule: "net/http-request-without-context"},
		{name: "net default client", root: fixtureRoot("net"), rule: "net/http-default-client"},
		{name: "net server timeouts", root: fixtureRoot("net"), rule: "net/http-server-no-timeouts"},
		{name: "time tick leak", root: fixtureRoot("time"), rule: "time/tick-leak"},
		{name: "time after in loop", root: fixtureRoot("time"), rule: "time/after-in-loop"},
		{name: "db rows close", root: fixtureRoot("db"), rule: "db/rows-not-closed"},
		{name: "db rows err", root: fixtureRoot("db"), rule: "db/rows-err-not-checked"},
		{name: "db tx rollback", root: fixtureRoot("db"), rule: "db/tx-no-deferred-rollback"},
		{name: "io readall", root: fixtureRoot("io"), rule: "io/readall-unbounded"},
		{name: "arch cross layer", root: fixtureRoot("arch-cross-layer"), rule: "arch/cross-layer-import"},
		{name: "arch cycles", root: fixtureRoot("arch-cycles"), rule: "arch/forbidden-package-cycles"},
		{name: "arch oversized package", root: fixtureRoot("arch-oversized"), rule: "arch/oversized-package"},
		{name: "arch god file", root: fixtureRoot("arch-god-file"), rule: "arch/god-file"},
		{name: "test missing table driven", root: fixtureRoot("test"), rule: "test/missing-table-driven"},
		{name: "test no assertions", root: fixtureRoot("test"), rule: "test/no-assertions"},
		{name: "test sleep", root: fixtureRoot("test"), rule: "test/sleep-in-test"},
		{name: "test handler no test", root: fixtureRoot("test"), rule: "test/http-handler-no-test"},
		{name: "sec math rand", root: fixtureRoot("sec"), rule: "sec/math-rand-for-secret"},
		{name: "sec insecure temp", root: fixtureRoot("sec"), rule: "sec/insecure-temp-file"},
		{name: "sec exec user input", root: fixtureRoot("sec"), rule: "sec/exec-user-input"},
		{name: "api error string branching", root: fixtureRoot("api"), rule: "api/error-string-branching"},
		{name: "api exported mutable global", root: fixtureRoot("api"), rule: "api/exported-mutable-global"},
		{name: "api init side effects", root: fixtureRoot("api"), rule: "api/init-side-effects"},
		{name: "lib os exit", root: fixtureRoot("lib"), rule: "lib/os-exit-in-non-main"},
		{name: "lib flag parse", root: fixtureRoot("lib"), rule: "lib/flag-parse-in-non-main"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			target := diagnostics.Target{
				RepoRoot:    testCase.root,
				Mode:        "module",
				ModuleRoots: []string{testCase.root},
			}
			analyzers := ExportDefaultAnalyzers(target, []string{testCase.rule}, nil)
			if len(analyzers) != 1 {
				t.Fatalf("expected one analyzer, got %d", len(analyzers))
			}

			got := analyzers[0].Run(context.Background(), target).Diagnostics
			wants := loadWants(t, testCase.root, testCase.rule)
			gotPairs := make([]string, 0, len(got))
			for _, diagnostic := range got {
				if diagnostic.Rule != testCase.rule {
					continue
				}
				gotPairs = append(gotPairs, diagnostic.Path+":"+itoa(diagnostic.Line))
			}
			slices.Sort(gotPairs)
			gotPairs = slices.Compact(gotPairs)
			if !slices.Equal(gotPairs, wants) {
				t.Fatalf("unexpected diagnostics for %s\nwant: %v\ngot:  %v", testCase.rule, wants, gotPairs)
			}
		})
	}
}

func TestCustomRuleGroups(t *testing.T) {
	root := fixtureRoot("context")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}
	analyzers := ExportDefaultAnalyzers(target, []string{"context"}, []string{"context/not-propagated"})
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}
	got := analyzers[0].Run(context.Background(), target).Diagnostics
	for _, diagnostic := range got {
		if diagnostic.Rule == "context/not-propagated" {
			t.Fatal("disabled rule still reported")
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 context diagnostics after disabling one rule, got %d", len(got))
	}
}

func TestCustomRuleSelectionIgnoresThirdPartySelectors(t *testing.T) {
	root := fixtureRoot("context")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"govet"}, nil)
	if len(analyzers) != 0 {
		t.Fatalf("expected no custom analyzer for third-party-only selection, got %d", len(analyzers))
	}

	analyzers = ExportDefaultAnalyzers(target, []string{"govet", "context/not-propagated"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one custom analyzer for mixed selection, got %d", len(analyzers))
	}
	got := analyzers[0].Run(context.Background(), target).Diagnostics
	if len(got) != 1 || got[0].Rule != "context/not-propagated" {
		t.Fatalf("expected only context/not-propagated finding, got %#v", got)
	}
}

func TestCustomRuleDefaultOff(t *testing.T) {
	root := fixtureRoot("io")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}
	analyzers := ExportDefaultAnalyzers(target, nil, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}
	got := analyzers[0].Run(context.Background(), target).Diagnostics
	for _, diagnostic := range got {
		switch diagnostic.Rule {
		case "io/readall-unbounded",
			"test/missing-table-driven",
			"test/no-assertions",
			"error/ignored-return",
			"context/missing-first-arg",
			"net/http-request-without-context",
			"perf/fmt-sprint-simple-concat":
			t.Fatalf("default-off rule reported without explicit enable: %s", diagnostic.Rule)
		}
	}
}

func TestDefaultOffRuleSelection(t *testing.T) {
	selected, err := selectRules(nil, nil)
	if err != nil {
		t.Fatalf("select rules: %v", err)
	}

	selectedSet := map[string]struct{}{}
	for _, rule := range selected {
		selectedSet[rule.desc.Rule] = struct{}{}
	}

	for _, ruleName := range []string{
		"io/readall-unbounded",
		"test/missing-table-driven",
		"test/no-assertions",
		"error/ignored-return",
		"context/missing-first-arg",
		"net/http-request-without-context",
		"perf/fmt-sprint-simple-concat",
	} {
		if _, ok := selectedSet[ruleName]; ok {
			t.Fatalf("expected %s to be default-off after calibration", ruleName)
		}
	}
}

func TestGeneratedFilesExcludedByDefault(t *testing.T) {
	root := fixtureRoot("generated")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}
	analyzers := ExportDefaultAnalyzers(target, []string{"error/ignored-return"}, nil)
	got := analyzers[0].Run(context.Background(), target).Diagnostics
	if len(got) != 0 {
		t.Fatalf("expected generated files to be skipped by default, got %d diagnostics", len(got))
	}

	target.IncludeGenerated = true
	got = analyzers[0].Run(context.Background(), target).Diagnostics
	if len(got) != 1 || got[0].Rule != "error/ignored-return" {
		t.Fatalf("expected generated file finding when includeGenerated is true, got %#v", got)
	}
}

func TestSelectedModuleInfosRejectsUnknownModuleFilter(t *testing.T) {
	target := diagnostics.Target{
		RepoRoot: filepath.Join("..", "..", "..", "testdata", "fixtures", "workspace"),
		Mode:     "workspace",
		ModuleRoots: []string{
			filepath.Join("..", "..", "..", "testdata", "fixtures", "workspace", "moda"),
			filepath.Join("..", "..", "..", "testdata", "fixtures", "workspace", "modb"),
		},
		ModulePatterns: []string{"missing"},
	}

	_, err := selectedModuleInfos(target)
	if err == nil {
		t.Fatal("expected unknown module filter to fail")
	}
	if !errors.Is(err, ErrNoModulesMatchedFilter) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRowsRuleDoesNotTreatErrMethodsAsRows(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"db/rows-not-closed"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	got := analyzers[0].Run(context.Background(), target).Diagnostics
	if len(got) != 0 {
		t.Fatalf("expected no db/rows-not-closed findings in the repo self-scan, got %#v", got)
	}
}

func TestPackageSelectionMissVisitsNoDirectories(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	target := diagnostics.Target{
		RepoRoot:        root,
		Mode:            "module",
		ModuleRoots:     []string{root},
		PackagePatterns: []string{"./missing"},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"arch/god-file", "api/exported-mutable-global"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics when package filter matches nothing, got %#v", result.Diagnostics)
	}
}

func TestCustomLayersOverrideBuiltInConvention(t *testing.T) {
	root := fixtureRoot("arch-cross-layer")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
		Architecture: []diagnostics.Layer{
			{Name: "domain", Include: []string{"internal/domain/..."}},
			{Name: "platform", Include: []string{"internal/platform/..."}},
			{Name: "transport", Include: []string{"internal/transport/..."}},
		},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"arch/cross-layer-import"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected custom layers to override built-in defaults, got %#v", result.Diagnostics)
	}
}

func TestNestedModulesAreExcludedFromModuleScan(t *testing.T) {
	root := fixtureRoot("nested-module")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"api/exported-mutable-global"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected nested module files to be excluded, got %#v", result.Diagnostics)
	}
}

func TestBuildTaggedFilesAreExcludedFromASTOnlyRules(t *testing.T) {
	root := fixtureRoot("build-tags")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"api/exported-mutable-global"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected build-tagged files outside the active build to be excluded, got %#v", result.Diagnostics)
	}
}

func TestBuildTaggedFilesFollowActiveGOFLAGSTags(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=tools")

	root := fixtureRoot("build-tags")
	target := diagnostics.Target{
		RepoRoot:    root,
		Mode:        "module",
		ModuleRoots: []string{root},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"api/exported-mutable-global"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Path != "tools.go" {
		t.Fatalf("expected tools-tagged file to be included when GOFLAGS enables it, got %#v", result.Diagnostics)
	}
}

func TestWorkspaceDefaultPatternsUseModuleRoots(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures", "workspace")
	target := diagnostics.Target{
		RepoRoot: root,
		Mode:     "workspace",
		ModuleRoots: []string{
			filepath.Join(root, "moda"),
			filepath.Join(root, "modb"),
		},
	}

	pass, toolErrors := loadAnalysisContext(context.Background(), target)
	if pass == nil {
		t.Fatalf("expected analysis context, got nil with tool errors %#v", toolErrors)
	}
	if len(toolErrors) != 0 {
		t.Fatalf("expected workspace load to succeed without tool errors, got %#v", toolErrors)
	}
}

func TestNestedSelectedWorkspaceModulesAreNotDuplicated(t *testing.T) {
	root := fixtureRoot("workspace-nested")
	target := diagnostics.Target{
		RepoRoot: root,
		Mode:     "workspace",
		ModuleRoots: []string{
			filepath.Join(root, "parent"),
			filepath.Join(root, "parent", "child"),
		},
	}

	analyzers := ExportDefaultAnalyzers(target, []string{"api/exported-mutable-global"}, nil)
	if len(analyzers) != 1 {
		t.Fatalf("expected one analyzer, got %d", len(analyzers))
	}

	result := analyzers[0].Run(context.Background(), target)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected exactly one finding for nested selected modules, got %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Path != "parent/child/child.go" {
		t.Fatalf("expected finding to come from child module file, got %#v", result.Diagnostics[0])
	}
}

func fixtureRoot(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "fixtures", "custom-rules", name)
}

func loadWants(t *testing.T, root string, rule string) []string {
	t.Helper()

	var wants []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		fileNode, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relative)
		for _, group := range fileNode.Comments {
			for _, comment := range group.List {
				if !strings.Contains(comment.Text, "want "+rule) {
					continue
				}
				position := fset.PositionFor(comment.Pos(), false)
				wants = append(wants, normalized+":"+itoa(position.Line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("load wants: %v", err)
	}
	slices.Sort(wants)
	wants = slices.Compact(wants)
	return wants
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
