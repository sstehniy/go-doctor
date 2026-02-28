package godoctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/stanislavstehniy/go-doctor/internal/analyzers/custom"
	"github.com/stanislavstehniy/go-doctor/internal/analyzers/repohygiene"
	"github.com/stanislavstehniy/go-doctor/internal/analyzers/thirdparty"
	"github.com/stanislavstehniy/go-doctor/internal/baseline"
	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
	"github.com/stanislavstehniy/go-doctor/internal/diff"
	"github.com/stanislavstehniy/go-doctor/internal/discovery"
	"github.com/stanislavstehniy/go-doctor/internal/model"
	"github.com/stanislavstehniy/go-doctor/internal/scoring"
	"github.com/stanislavstehniy/go-doctor/internal/suppressions"
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
	DiffGovulncheck  string
	Packages         []string
	Modules          []string
	Timeout          time.Duration
	Concurrency      int
	EnableRules      []string
	DisableRules     []string
	BaselinePath     string
	NoBaseline       bool
	RepoHygiene      bool
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

const (
	DiffGovulncheckSkip               = "skip"
	DiffGovulncheckChangedModulesOnly = "changed-modules-only"
)

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

	score := scoring.Score(nil, opts.Score)
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

	diffEnabled := strings.TrimSpace(opts.DiffBase) != ""
	diffPlan := diff.Plan{}
	if diffEnabled {
		base := opts.DiffBase
		if strings.TrimSpace(base) == "" {
			base = diff.AutoBase
		}
		diffPlan, err = diff.Discover(ctx, diff.Options{
			RepoRoot:    info.Root,
			ModuleRoots: info.ModuleRoots,
			Base:        base,
		})
		if err != nil {
			return DiagnoseResult{}, err
		}
		for _, warning := range diffPlan.Warnings {
			result.ToolErrors = append(result.ToolErrors, model.ToolError{
				Tool:    "diff",
				Message: warning,
			})
		}
		if diffPlan.Narrowed {
			targetSpec.PackagePatterns = append([]string(nil), diffPlan.PackagePatterns...)
			targetSpec.IncludeFiles = append([]string(nil), diffPlan.IncludeFiles...)
		}
	}

	analyzers := make([]diagnostics.Analyzer, 0, 8)
	if opts.RepoHygiene {
		for _, analyzer := range repohygiene.DefaultAnalyzers(targetSpec, opts.EnableRules, opts.DisableRules) {
			analyzers = append(analyzers, targetBoundAnalyzer{analyzer: analyzer, target: cloneTarget(targetSpec)})
		}
	} else {
		result.SkippedTools = append(result.SkippedTools, "repo-hygiene")
	}
	if opts.ThirdParty {
		for _, analyzer := range thirdparty.DefaultAnalyzers(targetSpec, opts.EnableRules, opts.DisableRules) {
			name := analyzer.Name()
			if diffEnabled && diffPlan.Narrowed {
				if isPackageScopedThirdParty(name) && len(diffPlan.PackagePatterns) == 0 {
					result.SkippedTools = append(result.SkippedTools, fmt.Sprintf("%s (diff: no changed packages)", name))
					continue
				}
				if name == "govulncheck" {
					if diffGovulncheckMode(opts.DiffGovulncheck) == DiffGovulncheckChangedModulesOnly {
						if len(diffPlan.ModulePatterns) == 0 {
							result.SkippedTools = append(result.SkippedTools, "govulncheck (diff: no changed modules)")
							continue
						}
						govulnTarget := cloneTarget(targetSpec)
						govulnTarget.ModulePatterns = append([]string(nil), diffPlan.ModulePatterns...)
						analyzers = append(analyzers, targetBoundAnalyzer{analyzer: analyzer, target: govulnTarget})
						continue
					}
					result.SkippedTools = append(result.SkippedTools, "govulncheck (diff default)")
					continue
				}
				if !analyzer.SupportsDiff() {
					result.SkippedTools = append(result.SkippedTools, fmt.Sprintf("%s (no diff support)", name))
					continue
				}
			}
			analyzers = append(analyzers, targetBoundAnalyzer{analyzer: analyzer, target: cloneTarget(targetSpec)})
		}
	} else {
		result.SkippedTools = append(result.SkippedTools, "third-party")
	}
	if opts.Custom {
		for _, analyzer := range custom.DefaultAnalyzers(targetSpec, opts.EnableRules, opts.DisableRules) {
			if diffEnabled && diffPlan.Narrowed && len(diffPlan.PackagePatterns) == 0 {
				result.SkippedTools = append(result.SkippedTools, "custom (diff: no changed packages)")
				continue
			}
			if diffEnabled && diffPlan.Narrowed && !analyzer.SupportsDiff() {
				result.SkippedTools = append(result.SkippedTools, fmt.Sprintf("%s (no diff support)", analyzer.Name()))
				continue
			}
			analyzers = append(analyzers, targetBoundAnalyzer{analyzer: analyzer, target: cloneTarget(targetSpec)})
		}
	} else {
		result.SkippedTools = append(result.SkippedTools, "custom")
	}

	perAnalyzerTimeout := defaultPerAnalyzerTimeout(opts.Timeout)
	diagnosticsOut, toolErrors := diagnostics.Run(ctx, analyzers, targetSpec, opts.Concurrency, perAnalyzerTimeout)
	result.Diagnostics = diagnosticsOut
	result.ToolErrors = append(result.ToolErrors, toolErrors...)
	if len(analyzers) > 0 && len(diagnosticsOut) == 0 && len(toolErrors) >= len(analyzers) {
		result.ElapsedMillis = time.Since(start).Milliseconds()
		return result, fmt.Errorf("all analyzers failed")
	}
	suppressionFilter, invalidSuppressions, suppressionToolErrors := suppressions.Load(targetSpec)
	result.Diagnostics = suppressions.Apply(result.Diagnostics, suppressionFilter)
	result.Diagnostics = append(result.Diagnostics, invalidSuppressions...)
	result.ToolErrors = append(result.ToolErrors, suppressionToolErrors...)
	sortDiagnostics(result.Diagnostics)
	sortToolErrors(result.ToolErrors)
	if err := applyBaseline(&result, opts); err != nil {
		result.ElapsedMillis = time.Since(start).Milliseconds()
		return result, err
	}
	score = scoring.Score(result.Diagnostics, opts.Score)
	result.ElapsedMillis = time.Since(start).Milliseconds()

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
	rules := repohygiene.SupportedRules()
	rules = append(rules, thirdparty.SupportedRules()...)
	rules = append(rules, custom.RuleNames()...)
	return uniqueSorted(rules)
}

func ListRuleSelectors() []string {
	selectors := repohygiene.SupportedSelectors()
	selectors = append(selectors, ListRules()...)
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

type targetBoundAnalyzer struct {
	analyzer diagnostics.Analyzer
	target   diagnostics.Target
}

func (t targetBoundAnalyzer) Name() string {
	return t.analyzer.Name()
}

func (t targetBoundAnalyzer) SupportsDiff() bool {
	return t.analyzer.SupportsDiff()
}

func (t targetBoundAnalyzer) Run(ctx context.Context, _ diagnostics.Target) diagnostics.Result {
	return t.analyzer.Run(ctx, t.target)
}

func cloneTarget(target diagnostics.Target) diagnostics.Target {
	cloned := diagnostics.Target{
		RepoRoot:         target.RepoRoot,
		Mode:             target.Mode,
		GoVersion:        target.GoVersion,
		ModuleRoots:      append([]string(nil), target.ModuleRoots...),
		PackagePatterns:  append([]string(nil), target.PackagePatterns...),
		ModulePatterns:   append([]string(nil), target.ModulePatterns...),
		IncludeFiles:     append([]string(nil), target.IncludeFiles...),
		IncludeGenerated: target.IncludeGenerated,
		Architecture:     make([]diagnostics.Layer, 0, len(target.Architecture)),
	}
	for _, layer := range target.Architecture {
		cloned.Architecture = append(cloned.Architecture, diagnostics.Layer{
			Name:    layer.Name,
			Include: append([]string(nil), layer.Include...),
			Allow:   append([]string(nil), layer.Allow...),
		})
	}
	return cloned
}

func isPackageScopedThirdParty(name string) bool {
	switch name {
	case "govet", "staticcheck", "golangci-lint":
		return true
	default:
		return false
	}
}

func diffGovulncheckMode(value string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	switch mode {
	case "", DiffGovulncheckSkip:
		return DiffGovulncheckSkip
	case DiffGovulncheckChangedModulesOnly:
		return DiffGovulncheckChangedModulesOnly
	default:
		return DiffGovulncheckSkip
	}
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

func applyBaseline(result *DiagnoseResult, opts Options) error {
	if opts.NoBaseline || opts.BaselinePath == "" {
		return nil
	}

	exists, err := baseline.Exists(opts.BaselinePath)
	if err != nil {
		return err
	}
	if exists {
		_, set, err := baseline.Load(opts.BaselinePath)
		if err != nil {
			return err
		}
		result.Diagnostics = baseline.Apply(result.Diagnostics, set)
		return nil
	}

	if ciEnabled() {
		return fmt.Errorf("baseline file %q does not exist in CI", opts.BaselinePath)
	}
	if len(result.ToolErrors) > 0 {
		return nil
	}
	if err := baseline.Write(opts.BaselinePath, result.Diagnostics); err != nil {
		return err
	}
	_, set, err := baseline.Load(opts.BaselinePath)
	if err != nil {
		return err
	}
	result.Diagnostics = baseline.Apply(result.Diagnostics, set)
	return nil
}

func ciEnabled() bool {
	value, ok := os.LookupEnv("CI")
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func sortDiagnostics(diagnosticsOut []model.Diagnostic) {
	sort.SliceStable(diagnosticsOut, func(i, j int) bool {
		left := diagnosticsOut[i]
		right := diagnosticsOut[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.Plugin != right.Plugin {
			return left.Plugin < right.Plugin
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Message < right.Message
	})
}

func sortToolErrors(toolErrors []model.ToolError) {
	sort.SliceStable(toolErrors, func(i, j int) bool {
		if toolErrors[i].Tool != toolErrors[j].Tool {
			return toolErrors[i].Tool < toolErrors[j].Tool
		}
		return toolErrors[i].Message < toolErrors[j].Message
	})
}
