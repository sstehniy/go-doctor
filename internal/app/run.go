package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/stanislavstehniy/go-doctor/internal/config"
	jsonoutput "github.com/stanislavstehniy/go-doctor/internal/output/json"
	textoutput "github.com/stanislavstehniy/go-doctor/internal/output/text"
	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
	ExitFatal   = 3
)

var Version = "dev"

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	cli, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}

	if cli.version {
		fmt.Fprintln(stdout, Version)
		return ExitSuccess
	}

	rules := godoctor.ListRules()
	if cli.listRules {
		if len(rules) == 0 {
			fmt.Fprintln(stdout, "no rules registered")
			return ExitSuccess
		}
		for _, rule := range rules {
			fmt.Fprintln(stdout, rule)
		}
		return ExitSuccess
	}

	opts := cfgpkg.DefaultOptions()
	configFile, _, err := cfgpkg.Load(cli.target, cli.configPath, rules)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	if err := configFile.Apply(&opts); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	applyCLIOverrides(&opts, cli)

	if err := cfgpkg.ValidateFormat(opts.Format); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	if err := cfgpkg.ValidateFailOn(opts.FailOn); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	if err := cfgpkg.ValidateRuleSelections(opts.EnableRules, opts.DisableRules, rules); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}

	result, err := godoctor.Diagnose(ctx, cli.target, opts)
	if err != nil {
		if result.SchemaVersion != 0 {
			rendered, renderErr := renderOutput(result, opts)
			if renderErr == nil && len(rendered) > 0 {
				if _, writeErr := stdout.Write(rendered); writeErr != nil {
					fmt.Fprintln(stderr, writeErr)
					return ExitFatal
				}
				if opts.OutputPath != "" {
					if writeErr := os.WriteFile(opts.OutputPath, rendered, 0o644); writeErr != nil {
						fmt.Fprintln(stderr, writeErr)
						return ExitFatal
					}
				}
			}
		}
		fmt.Fprintln(stderr, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return ExitFatal
		}
		return ExitFatal
	}

	rendered, err := renderOutput(result, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitFatal
	}
	if len(rendered) > 0 {
		if _, err := stdout.Write(rendered); err != nil {
			fmt.Fprintln(stderr, err)
			return ExitFatal
		}
	}
	if opts.OutputPath != "" && len(rendered) > 0 {
		if err := os.WriteFile(opts.OutputPath, rendered, 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return ExitFatal
		}
	}

	if breachesThreshold(result, opts.FailOn) {
		return ExitFailure
	}
	return ExitSuccess
}

type cliInput struct {
	target     string
	configPath string
	explicit   map[string]bool

	format      string
	output      string
	verbose     bool
	noScore     bool
	failOn      string
	diff        string
	packages    csvList
	modules     csvList
	timeout     durationFlag
	concurrency int
	enable      csvList
	disable     csvList
	baseline    string
	noBaseline  bool
	listRules   bool
	version     bool
	quiet       bool
}

func parseArgs(args []string) (cliInput, error) {
	cli := cliInput{
		target:   ".",
		explicit: map[string]bool{},
	}

	fs := flag.NewFlagSet("go-doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cli.configPath, "config", "", "")
	fs.StringVar(&cli.format, "format", "", "")
	fs.StringVar(&cli.output, "output", "", "")
	fs.BoolVar(&cli.verbose, "verbose", false, "")
	fs.BoolVar(&cli.noScore, "no-score", false, "")
	fs.StringVar(&cli.failOn, "fail-on", "", "")
	fs.StringVar(&cli.diff, "diff", "", "")
	fs.Var(&cli.packages, "packages", "")
	fs.Var(&cli.modules, "modules", "")
	fs.Var(&cli.timeout, "timeout", "")
	fs.IntVar(&cli.concurrency, "concurrency", 0, "")
	fs.Var(&cli.enable, "enable", "")
	fs.Var(&cli.disable, "disable", "")
	fs.StringVar(&cli.baseline, "baseline", "", "")
	fs.BoolVar(&cli.noBaseline, "no-baseline", false, "")
	fs.BoolVar(&cli.listRules, "list-rules", false, "")
	fs.BoolVar(&cli.version, "version", false, "")
	fs.BoolVar(&cli.quiet, "quiet", false, "")

	if err := fs.Parse(args); err != nil {
		return cli, err
	}
	fs.Visit(func(f *flag.Flag) {
		cli.explicit[f.Name] = true
	})

	remaining := fs.Args()
	if len(remaining) > 1 {
		return cli, fmt.Errorf("expected at most one target path")
	}
	if len(remaining) == 1 {
		cli.target = remaining[0]
	}

	return cli, nil
}

func applyCLIOverrides(opts *godoctor.Options, cli cliInput) {
	if cli.explicit["config"] {
		opts.ConfigPath = cli.configPath
	}
	if cli.explicit["format"] {
		opts.Format = cli.format
	}
	if cli.explicit["output"] {
		opts.OutputPath = cli.output
	}
	if cli.explicit["verbose"] {
		opts.Verbose = cli.verbose
	}
	if cli.explicit["no-score"] {
		opts.Score = !cli.noScore
	}
	if cli.explicit["fail-on"] {
		opts.FailOn = cli.failOn
	}
	if cli.explicit["diff"] {
		opts.DiffBase = cli.diff
	}
	if cli.explicit["packages"] {
		opts.Packages = append([]string(nil), cli.packages...)
	}
	if cli.explicit["modules"] {
		opts.Modules = append([]string(nil), cli.modules...)
	}
	if cli.explicit["timeout"] {
		opts.Timeout = time.Duration(cli.timeout.Duration)
	}
	if cli.explicit["concurrency"] && cli.concurrency > 0 {
		opts.Concurrency = cli.concurrency
	}
	if cli.explicit["enable"] {
		opts.EnableRules = append([]string(nil), cli.enable...)
	}
	if cli.explicit["disable"] {
		opts.DisableRules = append([]string(nil), cli.disable...)
	}
	if cli.explicit["baseline"] {
		opts.BaselinePath = cli.baseline
	}
	if cli.explicit["no-baseline"] {
		opts.NoBaseline = cli.noBaseline
	}
	if cli.explicit["quiet"] {
		opts.Quiet = cli.quiet
	}
}

func renderOutput(result godoctor.DiagnoseResult, opts godoctor.Options) ([]byte, error) {
	switch opts.Format {
	case "text":
		return textoutput.Render(result, opts.Verbose, opts.Quiet), nil
	case "json":
		return jsonoutput.Render(result)
	case "sarif":
		return godoctor.RenderSARIF(result)
	default:
		return nil, fmt.Errorf("unsupported output format %q", opts.Format)
	}
}

func breachesThreshold(result godoctor.DiagnoseResult, failOn string) bool {
	if failOn == "none" {
		return false
	}
	for _, diagnostic := range result.Diagnostics {
		if severityRank(diagnostic.Severity) >= severityRank(failOn) {
			return true
		}
	}
	return false
}

func severityRank(value string) int {
	switch strings.ToLower(value) {
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

type csvList []string

func (l *csvList) String() string {
	return strings.Join(*l, ",")
}

func (l *csvList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		*l = append(*l, item)
	}
	return nil
}

type durationFlag struct {
	Duration godoctor.Duration
}

func (d *durationFlag) String() string {
	return d.Duration.String()
}

func (d *durationFlag) Set(value string) error {
	return d.Duration.Set(value)
}
