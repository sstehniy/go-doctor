package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
	"gopkg.in/yaml.v3"
)

const (
	defaultTimeout = 30 * time.Second
)

type File struct {
	Scan         ScanConfig         `json:"scan" yaml:"scan"`
	Output       OutputConfig       `json:"output" yaml:"output"`
	CI           CIConfig           `json:"ci" yaml:"ci"`
	Rules        RulesConfig        `json:"rules" yaml:"rules"`
	Ignore       IgnoreConfig       `json:"ignore" yaml:"ignore"`
	Thresholds   ThresholdsConfig   `json:"thresholds" yaml:"thresholds"`
	Architecture ArchitectureConfig `json:"architecture" yaml:"architecture"`
	Analyzers    AnalyzersConfig    `json:"analyzers" yaml:"analyzers"`
}

type ScanConfig struct {
	Diff        string   `json:"diff" yaml:"diff"`
	Packages    []string `json:"packages" yaml:"packages"`
	Modules     []string `json:"modules" yaml:"modules"`
	Timeout     string   `json:"timeout" yaml:"timeout"`
	Concurrency int      `json:"concurrency" yaml:"concurrency"`
	Baseline    string   `json:"baseline" yaml:"baseline"`
	NoBaseline  bool     `json:"noBaseline" yaml:"noBaseline"`
}

type OutputConfig struct {
	Format  string `json:"format" yaml:"format"`
	Path    string `json:"path" yaml:"path"`
	Verbose bool   `json:"verbose" yaml:"verbose"`
	Score   *bool  `json:"score" yaml:"score"`
	Quiet   bool   `json:"quiet" yaml:"quiet"`
}

type CIConfig struct {
	FailOn string `json:"failOn" yaml:"failOn"`
}

type RulesConfig struct {
	Enable  []string `json:"enable" yaml:"enable"`
	Disable []string `json:"disable" yaml:"disable"`
}

type IgnoreConfig struct {
	Rules []string `json:"rules" yaml:"rules"`
	Paths []string `json:"paths" yaml:"paths"`
}

type ThresholdsConfig struct {
	Error   int `json:"error" yaml:"error"`
	Warning int `json:"warning" yaml:"warning"`
	Info    int `json:"info" yaml:"info"`
}

type ArchitectureConfig struct {
	Layers []Layer `json:"layers" yaml:"layers"`
}

type Layer struct {
	Name    string   `json:"name" yaml:"name"`
	Include []string `json:"include" yaml:"include"`
	Allow   []string `json:"allow" yaml:"allow"`
}

type AnalyzersConfig struct {
	ThirdParty *bool `json:"thirdParty" yaml:"thirdParty"`
	Custom     *bool `json:"custom" yaml:"custom"`
}

func DefaultOptions() godoctor.Options {
	return godoctor.Options{
		Format:      "text",
		Score:       true,
		FailOn:      "error",
		Timeout:     defaultTimeout,
		Concurrency: DefaultConcurrency(),
		ThirdParty:  true,
		Custom:      true,
	}
}

func DefaultConcurrency() int {
	cpu := runtime.NumCPU()
	if cpu < 1 {
		return 1
	}
	if cpu > 6 {
		return 6
	}
	return cpu
}

func Load(targetDir, explicitPath string, knownRules []string) (File, string, error) {
	path, err := findConfigPath(targetDir, explicitPath)
	if err != nil {
		return File{}, "", err
	}
	if path == "" {
		return File{}, "", nil
	}
	cfg, err := readFile(path)
	if err != nil {
		return File{}, "", err
	}
	if err := validateRules(cfg, knownRules); err != nil {
		return File{}, "", err
	}
	return cfg, filepath.Clean(path), nil
}

func (f File) Apply(opts *godoctor.Options) error {
	if f.Scan.Diff != "" {
		opts.DiffBase = f.Scan.Diff
	}
	if len(f.Scan.Packages) > 0 {
		opts.Packages = slices.Clone(f.Scan.Packages)
	}
	if len(f.Scan.Modules) > 0 {
		opts.Modules = slices.Clone(f.Scan.Modules)
	}
	if f.Scan.Timeout != "" {
		timeout, err := time.ParseDuration(f.Scan.Timeout)
		if err != nil {
			return fmt.Errorf("parse config scan.timeout: %w", err)
		}
		opts.Timeout = timeout
	}
	if f.Scan.Concurrency > 0 {
		opts.Concurrency = f.Scan.Concurrency
	}
	if f.Scan.Baseline != "" {
		opts.BaselinePath = f.Scan.Baseline
	}
	if f.Scan.NoBaseline {
		opts.NoBaseline = true
	}
	if f.Output.Format != "" {
		opts.Format = f.Output.Format
	}
	if f.Output.Path != "" {
		opts.OutputPath = f.Output.Path
	}
	if f.Output.Verbose {
		opts.Verbose = true
	}
	if f.Output.Score != nil {
		opts.Score = *f.Output.Score
	}
	if f.Output.Quiet {
		opts.Quiet = true
	}
	if f.CI.FailOn != "" {
		opts.FailOn = f.CI.FailOn
	}
	if len(f.Rules.Enable) > 0 {
		opts.EnableRules = slices.Clone(f.Rules.Enable)
	}
	if len(f.Rules.Disable) > 0 {
		opts.DisableRules = slices.Clone(f.Rules.Disable)
	}
	if f.Analyzers.ThirdParty != nil {
		opts.ThirdParty = *f.Analyzers.ThirdParty
	}
	if f.Analyzers.Custom != nil {
		opts.Custom = *f.Analyzers.Custom
	}
	return nil
}

func findConfigPath(targetDir, explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return "", fmt.Errorf("read config %q: %w", explicitPath, err)
		}
		return explicitPath, nil
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("resolve target %q: %w", targetDir, err)
	}
	candidates := []string{
		filepath.Join(absTarget, ".go-doctor.yaml"),
		filepath.Join(absTarget, ".go-doctor.yml"),
		filepath.Join(absTarget, ".go-doctor.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func readFile(path string) (File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read config %q: %w", path, err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return parseYAML(raw)
	case ".json":
		return parseJSON(raw)
	default:
		return File{}, fmt.Errorf("unsupported config file type %q", filepath.Ext(path))
	}
}

func parseYAML(raw []byte) (File, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var cfg File
	if err := decoder.Decode(&cfg); err != nil {
		return File{}, fmt.Errorf("parse yaml config: %w", err)
	}
	return cfg, nil
}

func parseJSON(raw []byte) (File, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var cfg File
	if err := decoder.Decode(&cfg); err != nil {
		return File{}, fmt.Errorf("parse json config: %w", err)
	}
	return cfg, nil
}

func validateRules(cfg File, knownRules []string) error {
	known := map[string]struct{}{}
	for _, rule := range knownRules {
		known[rule] = struct{}{}
	}

	validate := func(source string, names []string) error {
		for _, name := range names {
			if _, ok := known[name]; ok {
				continue
			}
			return fmt.Errorf("%s references unknown rule %q", source, name)
		}
		return nil
	}

	if err := validate("rules.enable", cfg.Rules.Enable); err != nil {
		return err
	}
	if err := validate("rules.disable", cfg.Rules.Disable); err != nil {
		return err
	}
	if err := validate("ignore.rules", cfg.Ignore.Rules); err != nil {
		return err
	}
	return nil
}

func ValidateFailOn(value string) error {
	switch value {
	case "none", "info", "warning", "error":
		return nil
	default:
		return fmt.Errorf("unsupported --fail-on value %q", value)
	}
}

func ValidateFormat(value string) error {
	switch value {
	case "text", "json", "sarif":
		return nil
	default:
		return fmt.Errorf("unsupported --format value %q", value)
	}
}

func ValidateRuleSelections(enable, disable, knownRules []string) error {
	cfg := File{
		Rules: RulesConfig{
			Enable:  enable,
			Disable: disable,
		},
	}
	return validateRules(cfg, knownRules)
}

func IsUsageError(err error) bool {
	return errors.Is(err, ErrUsage)
}

var ErrUsage = errors.New("usage error")
