package godoctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHardeningIntegrationFixturesExist(t *testing.T) {
	required := []string{
		"clean-service",
		"basic-api-smells",
		"workspace-multi-module",
		"vuln-demo",
		"legacy-monolith",
		"diff-mode-changes",
		"http-hardening-demo",
		"sql-leaks-demo",
		"mod-hygiene-demo",
		"timers-leaks-demo",
		"http-hardening-clean",
		"sql-clean",
		"mod-clean",
	}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			path := integrationFixturePath(t, name)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("expected fixture %s to exist: %v", name, err)
			}
			if !info.IsDir() {
				t.Fatalf("expected fixture %s to be a directory", name)
			}
		})
	}
}

func TestHardeningIntegrationFixtureScans(t *testing.T) {
	testCases := []struct {
		name         string
		enableRules  []string
		repoHygiene  bool
		custom       bool
		wantMinCount int
	}{
		{name: "basic-api-smells", enableRules: []string{"error/string-compare", "api/exported-mutable-global"}, custom: true, wantMinCount: 1},
		{name: "http-hardening-demo", enableRules: []string{"net/http-default-client", "net/http-server-no-timeouts"}, custom: true, wantMinCount: 1},
		{name: "sql-leaks-demo", enableRules: []string{"db/rows-not-closed", "db/rows-err-not-checked"}, custom: true, wantMinCount: 1},
		{name: "timers-leaks-demo", enableRules: []string{"time/tick-leak", "time/after-in-loop"}, custom: true, wantMinCount: 1},
		{name: "legacy-monolith", enableRules: []string{"arch/god-file"}, custom: true, wantMinCount: 1},
		{name: "mod-hygiene-demo", enableRules: []string{"mod/replace-local-path"}, repoHygiene: true, wantMinCount: 1},
		{name: "http-hardening-clean", enableRules: []string{"net/http-default-client", "net/http-server-no-timeouts"}, custom: true, wantMinCount: 0},
		{name: "sql-clean", enableRules: []string{"db/rows-not-closed", "db/rows-err-not-checked"}, custom: true, wantMinCount: 0},
		{name: "mod-clean", enableRules: []string{"mod/replace-local-path", "mod/not-tidy"}, repoHygiene: true, wantMinCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := Diagnose(context.Background(), integrationFixturePath(t, testCase.name), Options{
				Timeout:      20 * time.Second,
				Concurrency:  2,
				EnableRules:  testCase.enableRules,
				RepoHygiene:  testCase.repoHygiene,
				ThirdParty:   false,
				Custom:       testCase.custom,
				Score:        false,
				BaselinePath: "",
			})
			if err != nil {
				t.Fatalf("diagnose fixture %s: %v", testCase.name, err)
			}
			if len(result.Diagnostics) < testCase.wantMinCount {
				t.Fatalf("expected at least %d diagnostics, got %#v", testCase.wantMinCount, result.Diagnostics)
			}
			if testCase.wantMinCount == 0 && len(result.Diagnostics) != 0 {
				t.Fatalf("expected clean fixture, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestHardeningWorkspaceAndDiffFixturesDiscover(t *testing.T) {
	for _, name := range []string{"workspace-multi-module", "vuln-demo", "diff-mode-changes", "clean-service"} {
		t.Run(name, func(t *testing.T) {
			result, err := Diagnose(context.Background(), integrationFixturePath(t, name), Options{
				Timeout:      20 * time.Second,
				Concurrency:  1,
				RepoHygiene:  false,
				ThirdParty:   false,
				Custom:       false,
				Score:        false,
				BaselinePath: "",
			})
			if err != nil {
				t.Fatalf("diagnose fixture %s: %v", name, err)
			}
			if name == "workspace-multi-module" && result.Project.Mode != "workspace" {
				t.Fatalf("expected workspace discovery mode, got %#v", result.Project)
			}
		})
	}
}

func integrationFixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures", "integration", name)
}
