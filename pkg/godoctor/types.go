package godoctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/stanislavstehniy/go-doctor/internal/analyzers/custom"
	"github.com/stanislavstehniy/go-doctor/internal/analyzers/thirdparty"
	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/discovery"
	"github.com/stanislavstehniy/go-doctor/internal/model"
	"github.com/stanislavstehniy/go-doctor/internal/scoring"
)

type Options struct {
	ConfigPath       string
	Format           string
	OutputPath       string
	Verbose          bool
	Quiet            bool
	Score            bool
	FailOn           string
	DiffBase         string
	Packages         []string
	Modules          []string
	Timeout          time.Duration
	Concurrency      int
	EnableRules      []string
	DisableRules     []string
	BaselinePath     string
	NoBaseline       bool
	ThirdParty       bool
	Custom           bool
	IncludeGenerated bool
	Architecture     []Layer
}

type Layer struct {
	Name    string
	Include []string
	Allow   []string
}

type Diagnostic = model.Diagnostic

type ProjectInfo struct {
	Target       string   `json:"target"`
	Root         string   `json:"root"`
	Mode         string   `json:"mode"`
	GoVersion    string   `json:"goVersion,omitempty"`
	ModuleRoots  []string `json:"moduleRoots"`
	PackageCount int      `json:"packageCount"`
}

type ScoreResult struct {
	Enabled bool   `json:"enabled"`
	Value   int    `json:"value,omitempty"`
	Max     int    `json:"max,omitempty"`
	Grade   string `json:"grade,omitempty"`
}

type ToolError = model.ToolError

type DiagnoseResult struct {
	SchemaVersion int          `json:"schemaVersion"`
	Project       ProjectInfo  `json:"project"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Score         *ScoreResult `json:"score,omitempty"`
	SkippedTools  []string     `json:"skippedTools,omitempty"`
	ToolErrors    []ToolError  `json:"toolErrors,omitempty"`
	ElapsedMillis int64        `json:"elapsedMillis"`
}

func Diagnose(ctx context.Context, target string, opts Options) (DiagnoseResult, error) {
	start := time.Now()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		return DiagnoseResult{}, ctx.Err()
	default:
	}

	info, err := discovery.Discover(target)
	if err != nil {
		return DiagnoseResult{}, err
	}

	score := scoring.Score(0, opts.Score)
	result := DiagnoseResult{
		SchemaVersion: 1,
		Project: ProjectInfo{
			Target:       info.Target,
			Root:         info.Root,
			Mode:         info.Mode,
			GoVersion:    info.GoVersion,
			ModuleRoots:  append([]string(nil), info.ModuleRoots...),
			PackageCount: info.PackageCount,
		},
		Diagnostics:   []Diagnostic{},
		SkippedTools:  []string{},
		ToolErrors:    []ToolError{},
		ElapsedMillis: time.Since(start).Milliseconds(),
	}

	targetSpec := diagnostics.Target{
		RepoRoot:         info.Root,
		Mode:             info.Mode,
		GoVersion:        info.GoVersion,
		ModuleRoots:      append([]string(nil), info.ModuleRoots...),
		PackagePatterns:  append([]string(nil), opts.Packages...),
		ModulePatterns:   append([]string(nil), opts.Modules...),
		IncludeGenerated: opts.IncludeGenerated,
		Architecture:     make([]diagnostics.Layer, 0, len(opts.Architecture)),
	}
	for _, layer := range opts.Architecture {
		targetSpec.Architecture = append(targetSpec.Architecture, diagnostics.Layer{
			Name:    layer.Name,
			Include: append([]string(nil), layer.Include...),
			Allow:   append([]string(nil), layer.Allow...),
		})
	}
	analyzers := make([]diagnostics.Analyzer, 0, 4)
	if opts.ThirdParty {
		analyzers = append(analyzers, thirdparty.DefaultAnalyzers(targetSpec, opts.EnableRules, opts.DisableRules)...)
	} else {
		result.SkippedTools = append(result.SkippedTools, "third-party")
	}
	if opts.Custom {
		analyzers = append(analyzers, custom.DefaultAnalyzers(targetSpec, opts.EnableRules, opts.DisableRules)...)
	} else {
		result.SkippedTools = append(result.SkippedTools, "custom")
	}

	perAnalyzerTimeout := defaultPerAnalyzerTimeout(opts.Timeout)
	diagnosticsOut, toolErrors := diagnostics.Run(ctx, analyzers, targetSpec, opts.Concurrency, perAnalyzerTimeout)
	result.Diagnostics = diagnosticsOut
	result.ToolErrors = toolErrors
	score = scoring.Score(len(diagnosticsOut), opts.Score)
	result.ElapsedMillis = time.Since(start).Milliseconds()
	if len(analyzers) > 0 && len(diagnosticsOut) == 0 && len(toolErrors) >= len(analyzers) {
		return result, fmt.Errorf("all analyzers failed")
	}

	if score.Enabled {
		result.Score = &ScoreResult{
			Enabled: true,
			Value:   score.Value,
			Max:     score.Max,
			Grade:   score.Grade,
		}
	} else {
		result.Score = &ScoreResult{Enabled: false}
	}
	return result, nil
}

func ListRules() []string {
	rules := thirdparty.SupportedRules()
	rules = append(rules, custom.RuleNames()...)
	return uniqueSorted(rules)
}

func ListRuleSelectors() []string {
	selectors := ListRules()
	selectors = append(selectors, custom.SelectorNames()...)
	return uniqueSorted(selectors)
}

func RenderSARIF(result DiagnoseResult) ([]byte, error) {
	return nil, errors.New("sarif output is reserved for a later milestone")
}

type Duration time.Duration

func (d *Duration) String() string {
	return time.Duration(*d).String()
}

func (d *Duration) Set(value string) error {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value, err)
	}
	*d = Duration(parsed)
	return nil
}

func NormalizePath(path string) string {
	return model.NormalizePath(path)
}

func defaultPerAnalyzerTimeout(total time.Duration) time.Duration {
	if total <= 0 {
		return 0
	}
	return total
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
