package godoctor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/stanislavstehniy/go-doctor/internal/discovery"
	"github.com/stanislavstehniy/go-doctor/internal/scoring"
)

type Options struct {
	ConfigPath   string
	Format       string
	OutputPath   string
	Verbose      bool
	Quiet        bool
	Score        bool
	FailOn       string
	DiffBase     string
	Packages     []string
	Modules      []string
	Timeout      time.Duration
	Concurrency  int
	EnableRules  []string
	DisableRules []string
	BaselinePath string
	NoBaseline   bool
}

type Diagnostic struct {
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`

	Plugin   string `json:"plugin,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`

	Message string `json:"message"`
	Help    string `json:"help,omitempty"`

	Symbol     string `json:"symbol,omitempty"`
	Package    string `json:"package,omitempty"`
	Module     string `json:"module,omitempty"`
	DocsURL    string `json:"docsUrl,omitempty"`
	Weight     int    `json:"weight,omitempty"`
	Suppressed bool   `json:"suppressed,omitempty"`
}

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

type ToolError struct {
	Tool    string `json:"tool"`
	Message string `json:"message"`
	Fatal   bool   `json:"fatal,omitempty"`
}

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
	return nil
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
	return filepath.ToSlash(filepath.Clean(path))
}
