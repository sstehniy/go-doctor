package sarif

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sstehniy/go-doctor/internal/model"
)

func TestRenderEmitsSARIF21WithSingleRun(t *testing.T) {
	root := filepath.FromSlash("/repo")
	if runtime.GOOS == "windows" {
		root = `C:\repo`
	}
	absMain := filepath.Join(root, "pkg", "main.go")

	body, err := Render(Input{
		ProjectRoot: root,
		Diagnostics: []model.Diagnostic{
			{
				Path:     absMain,
				Line:     12,
				Column:   3,
				Plugin:   "custom",
				Rule:     "error/string-compare",
				Severity: "error",
				Category: "correctness",
				Message:  "error compared to a string value",
				Help:     "Compare errors with errors.Is/errors.As.",
				DocsURL:  "https://example.com/custom/error/string-compare",
			},
			{
				Path:       "pkg/helper.go",
				Line:       4,
				Column:     1,
				Plugin:     "repo",
				Rule:       "fmt/not-gofmt",
				Severity:   "info",
				Category:   "fmt",
				Message:    "file is not gofmt formatted",
				Help:       "Run gofmt on the reported files before committing.",
				Suppressed: true,
			},
			{
				Path:     "pkg/adapter.go",
				Line:     6,
				Column:   2,
				Plugin:   "staticcheck",
				Rule:     "SA1000",
				Severity: "warning",
				Category: "correctness",
				Message:  "invalid regexp",
			},
		},
		ToolErrors: []model.ToolError{
			{Tool: "diff", Message: "remote default branch not found"},
			{Tool: "custom", Message: "timed out", Fatal: true},
		},
		RuleMetadata: []RuleMetadata{
			{
				Plugin:      "custom",
				Rule:        "error/string-compare",
				Description: "Error string comparison",
				Help:        "Compare errors with errors.Is/errors.As.",
				DocsURL:     "https://example.com/custom/error/string-compare",
				Severity:    "error",
				Category:    "correctness",
			},
		},
		ScoreEnabled: true,
		ScoreValue:   88,
		ScoreMax:     100,
		ScoreGrade:   "Good",
	})
	if err != nil {
		t.Fatalf("render sarif: %v", err)
	}

	var log struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID      string `json:"id"`
						HelpURI string `json:"helpUri"`
						Help    struct {
							Text string `json:"text"`
						} `json:"help"`
						DefaultConfiguration struct {
							Level string `json:"level"`
						} `json:"defaultConfiguration"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				Suppressions []struct {
					Status string `json:"status"`
				} `json:"suppressions"`
			} `json:"results"`
			Invocations []struct {
				ExecutionSuccessful        bool `json:"executionSuccessful"`
				ToolExecutionNotifications []struct {
					Level string `json:"level"`
				} `json:"toolExecutionNotifications"`
			} `json:"invocations"`
			Properties map[string]any `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(body, &log); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}

	if log.Version != "2.1.0" {
		t.Fatalf("unexpected version: %q", log.Version)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("expected one run, got %d", len(log.Runs))
	}
	if log.Runs[0].Tool.Driver.Name != "go-doctor" {
		t.Fatalf("unexpected driver: %q", log.Runs[0].Tool.Driver.Name)
	}
	if len(log.Runs[0].Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(log.Runs[0].Results))
	}

	levels := map[string]string{}
	for _, result := range log.Runs[0].Results {
		levels[result.RuleID] = result.Level
	}
	if levels["custom/error/string-compare"] != "error" {
		t.Fatalf("expected error mapping, got %q", levels["custom/error/string-compare"])
	}
	if levels["repo/fmt/not-gofmt"] != "note" {
		t.Fatalf("expected info->note mapping, got %q", levels["repo/fmt/not-gofmt"])
	}
	if levels["staticcheck/SA1000"] != "warning" {
		t.Fatalf("expected warning mapping, got %q", levels["staticcheck/SA1000"])
	}

	if got := log.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI; got == "" || got[0] == '/' {
		t.Fatalf("expected relative artifact path, got %q", got)
	}

	helpByRule := map[string]struct {
		helpURI string
		help    string
		level   string
	}{}
	for _, rule := range log.Runs[0].Tool.Driver.Rules {
		helpByRule[rule.ID] = struct {
			helpURI string
			help    string
			level   string
		}{
			helpURI: rule.HelpURI,
			help:    rule.Help.Text,
			level:   rule.DefaultConfiguration.Level,
		}
	}
	if helpByRule["custom/error/string-compare"].help == "" {
		t.Fatal("expected help text in rule metadata")
	}
	if helpByRule["custom/error/string-compare"].helpURI == "" {
		t.Fatal("expected docs url in rule metadata")
	}
	if helpByRule["custom/error/string-compare"].level != "error" {
		t.Fatalf("expected default rule level error, got %q", helpByRule["custom/error/string-compare"].level)
	}

	if len(log.Runs[0].Invocations) != 1 {
		t.Fatalf("expected invocation data, got %d", len(log.Runs[0].Invocations))
	}
	if log.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Fatal("expected fatal tool error to mark invocation unsuccessful")
	}
	if len(log.Runs[0].Invocations[0].ToolExecutionNotifications) != 2 {
		t.Fatalf("expected tool notifications, got %d", len(log.Runs[0].Invocations[0].ToolExecutionNotifications))
	}

	if _, ok := log.Runs[0].Properties["goDoctorScore"]; !ok {
		t.Fatal("expected score properties in SARIF run")
	}
}

func TestRenderDropsAbsolutePathsOutsideRepo(t *testing.T) {
	root := filepath.FromSlash("/repo")
	outside := filepath.FromSlash("/outside/main.go")
	if runtime.GOOS == "windows" {
		root = `C:\repo`
		outside = `D:\outside\main.go`
	}

	body, err := Render(Input{
		ProjectRoot: root,
		Diagnostics: []model.Diagnostic{
			{
				Path:     outside,
				Line:     1,
				Column:   1,
				Plugin:   "custom",
				Rule:     "arch/god-file",
				Severity: "warning",
				Message:  "god file",
			},
		},
	})
	if err != nil {
		t.Fatalf("render sarif: %v", err)
	}

	var log struct {
		Runs []struct {
			Results []struct {
				Locations []json.RawMessage `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(body, &log); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	if len(log.Runs) != 1 || len(log.Runs[0].Results) != 1 {
		t.Fatalf("unexpected sarif results shape: %#v", log)
	}
	if len(log.Runs[0].Results[0].Locations) != 0 {
		t.Fatalf("expected location to be omitted for outside absolute path, got %d", len(log.Runs[0].Results[0].Locations))
	}
}

func TestRenderDropsRelativePathsOutsideRepo(t *testing.T) {
	body, err := Render(Input{
		ProjectRoot: filepath.FromSlash("/repo"),
		Diagnostics: []model.Diagnostic{
			{
				Path:     "../outside/main.go",
				Line:     1,
				Column:   1,
				Plugin:   "custom",
				Rule:     "arch/god-file",
				Severity: "warning",
				Message:  "god file",
			},
		},
	})
	if err != nil {
		t.Fatalf("render sarif: %v", err)
	}

	var log struct {
		Runs []struct {
			Results []struct {
				Locations []json.RawMessage `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(body, &log); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	if len(log.Runs) != 1 || len(log.Runs[0].Results) != 1 {
		t.Fatalf("unexpected sarif results shape: %#v", log)
	}
	if len(log.Runs[0].Results[0].Locations) != 0 {
		t.Fatalf("expected location to be omitted for escaping relative path, got %d", len(log.Runs[0].Results[0].Locations))
	}
}
