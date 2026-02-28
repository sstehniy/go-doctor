package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const usageHint = "Run 'go-doctor --help' for usage."

type usageError struct {
	cause error
}

func (err *usageError) Error() string {
	if err == nil || err.cause == nil {
		return ""
	}
	return err.cause.Error()
}

func (err *usageError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	return &usageError{cause: err}
}

func newUsageErrorf(format string, args ...any) error {
	return &usageError{cause: fmt.Errorf(format, args...)}
}

type commandState struct {
	ctx      context.Context
	stdout   io.Writer
	stderr   io.Writer
	cli      cliInput
	exitCode int
}

type flagGroup struct {
	title string
	names []string
}

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

var rootFlagGroups = []flagGroup{
	{title: "Common Flags", names: []string{"help", "config", "version", "list-rules"}},
	{title: "Output Flags", names: []string{"format", "output", "verbose", "quiet", "no-score", "fail-on"}},
	{title: "Scope Flags", names: []string{"diff", "diff-govulncheck", "packages", "modules"}},
	{title: "Rule Flags", names: []string{"enable", "disable"}},
	{title: "Baseline And CI Flags", names: []string{"baseline", "no-baseline"}},
	{title: "Advanced Flags", names: []string{"timeout", "concurrency"}},
}

func newCommandState(ctx context.Context, stdout io.Writer, stderr io.Writer) *commandState {
	return &commandState{
		ctx:    ctx,
		stdout: stdout,
		stderr: stderr,
		cli: cliInput{
			target:   ".",
			explicit: map[string]bool{},
		},
		exitCode: ExitSuccess,
	}
}

func NewRootCommand(ctx context.Context, stdout io.Writer, stderr io.Writer) *cobra.Command {
	return newCommandState(ctx, stdout, stderr).rootCommand()
}

func (state *commandState) rootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "go-doctor [flags] [target]",
		Short:         "Check Go repositories for code health issues.",
		Long:          "go-doctor scans Go repositories, normalizes findings across analyzers, and reports a clear health signal for local work and CI.",
		Example:       rootExamples(),
		Args:          state.validateRootArgs,
		RunE:          state.runRoot,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SetOut(state.stdout)
	cmd.SetErr(state.stderr)
	cmd.SetContext(state.ctx)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return newUsageError(err)
	})

	flags := cmd.Flags()
	flags.SortFlags = false
	flags.StringVar(&state.cli.configPath, "config", "", "Path to config file. Defaults to auto-discovery in the target.")
	flags.StringVar(&state.cli.format, "format", "", "Output format: text, json, or sarif. Defaults to config or text.")
	flags.StringVar(&state.cli.output, "output", "", "Write rendered output to this path in addition to stdout.")
	flags.BoolVar(&state.cli.verbose, "verbose", false, "Show verbose output in text mode.")
	flags.BoolVar(&state.cli.quiet, "quiet", false, "Reduce summary noise in text mode.")
	flags.BoolVar(&state.cli.noScore, "no-score", false, "Disable score output, overriding config.")
	flags.StringVar(&state.cli.failOn, "fail-on", "", "Fail threshold: none, info, warning, or error.")
	flags.StringVar(&state.cli.diff, "diff", "", "Diff base ('auto' or an explicit ref). Bare --diff uses auto.")
	flags.StringVar(&state.cli.diffGovuln, "diff-govulncheck", "", "Diff govulncheck mode: skip or changed-modules-only.")
	flags.Var(&state.cli.packages, "packages", "Comma-separated package patterns to scan.")
	flags.Var(&state.cli.modules, "modules", "Comma-separated module roots to scan.")
	flags.Var(&state.cli.timeout, "timeout", "Global scan timeout, such as 30s or 2m.")
	flags.IntVar(&state.cli.concurrency, "concurrency", 0, "Max analyzer concurrency. Defaults to config or CPU-based default.")
	flags.Var(&state.cli.enable, "enable", "Comma-separated rules or selectors to enable.")
	flags.Var(&state.cli.disable, "disable", "Comma-separated rules or selectors to disable.")
	flags.StringVar(&state.cli.baseline, "baseline", "", "Baseline file path for adoption workflows.")
	flags.BoolVar(&state.cli.noBaseline, "no-baseline", false, "Ignore any configured baseline for this run.")
	flags.BoolVar(&state.cli.listRules, "list-rules", false, "List available rules and selectors, then exit.")
	flags.BoolVar(&state.cli.version, "version", false, "Print version and exit.")

	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultHelpCmd()
	cmd.AddCommand(state.newCompletionCommand())
	cmd.SetHelpFunc(renderRootHelp)
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		renderRootHelp(cmd, nil)
		return nil
	})

	return cmd
}

func rootExamples() string {
	lines := []string{
		"go-doctor .",
		"go-doctor --format json .",
		"go-doctor --format sarif --output results.sarif .",
		"go-doctor --diff .",
		"go-doctor --diff origin/main .",
		"go-doctor --list-rules",
		"go-doctor --baseline .go-doctor-baseline.json --fail-on warning .",
		"go-doctor completion",
		"go-doctor completion zsh",
	}
	return strings.Join(lines, "\n")
}

func (state *commandState) validateRootArgs(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return newUsageErrorf("expected at most one target path")
	}
	if len(args) == 1 {
		state.cli.target = args[0]
	}
	return nil
}

func (state *commandState) runRoot(cmd *cobra.Command, _ []string) error {
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		state.cli.explicit[flag.Name] = true
	})
	return state.executeRoot()
}

func (state *commandState) newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "completion [bash|zsh|fish|powershell]",
		Short:         "Show shell completion install instructions.",
		Long:          "Show concise installation instructions for shell completions. Run without a shell to see every supported shell, or use 'go-doctor completion script <shell>' to print the raw script.",
		Example:       "go-doctor completion\ngo-doctor completion zsh\ngo-doctor completion script zsh > ~/.zsh/completions/_go-doctor",
		Args:          validateCompletionGuideArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          state.runCompletionGuide,
	}
	cmd.ValidArgs = append([]string(nil), completionShells...)
	cmd.AddCommand(state.newCompletionScriptCommand())
	return cmd
}

func validateCompletionGuideArgs(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return newUsageErrorf("accepts at most 1 arg(s), received %d", len(args))
	}
	return nil
}

func (state *commandState) runCompletionGuide(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		_, _ = io.WriteString(state.stdout, renderCompletionGuide(cmd.Root().Name(), ""))
		return nil
	}

	shell, err := normalizeCompletionShell(args[0])
	if err != nil {
		return err
	}

	_, _ = io.WriteString(state.stdout, renderCompletionGuide(cmd.Root().Name(), shell))
	return nil
}

func (state *commandState) newCompletionScriptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "script <bash|zsh|fish|powershell>",
		Short:         "Print the raw shell completion script.",
		Long:          "Print the raw shell completion script for a specific shell. This is intended for redirection into a completion file or sourcing in your shell startup flow.",
		Example:       "go-doctor completion script bash\ngo-doctor completion script zsh > ~/.zsh/completions/_go-doctor",
		Args:          validateCompletionScriptArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          state.runCompletionScript,
	}
	cmd.ValidArgs = append([]string(nil), completionShells...)
	return cmd
}

func validateCompletionScriptArgs(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return newUsageErrorf("accepts 1 arg(s), received %d", len(args))
	}
	return nil
}

func (state *commandState) runCompletionScript(cmd *cobra.Command, args []string) error {
	shell, err := normalizeCompletionShell(args[0])
	if err != nil {
		return err
	}

	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletionV2(state.stdout, true)
	case "zsh":
		return cmd.Root().GenZshCompletion(state.stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(state.stdout, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletionWithDesc(state.stdout)
	default:
		return newUsageErrorf("unsupported shell %q", args[0])
	}
}

func normalizeCompletionShell(value string) (string, error) {
	shell := strings.ToLower(strings.TrimSpace(value))
	switch shell {
	case "bash", "zsh", "fish", "powershell":
		return shell, nil
	default:
		return "", newUsageErrorf("unsupported shell %q", value)
	}
}

func renderCompletionGuide(rootName string, shell string) string {
	if shell == "" {
		var builder strings.Builder
		builder.WriteString("Shell completion install guide.\n\n")
		builder.WriteString("Run `")
		builder.WriteString(rootName)
		builder.WriteString(" completion <shell>` for one shell only.\n")
		builder.WriteString("Run `")
		builder.WriteString(rootName)
		builder.WriteString(" completion script <shell>` to print the raw script.\n\n")
		for index, current := range completionShells {
			if index > 0 {
				builder.WriteString("\n\n")
			}
			builder.WriteString(renderCompletionShellGuide(rootName, current))
		}
		builder.WriteByte('\n')
		return builder.String()
	}
	return renderCompletionShellGuide(rootName, shell) + "\n"
}

func renderCompletionShellGuide(rootName string, shell string) string {
	scriptCommand := rootName + " completion script " + shell

	switch shell {
	case "bash":
		return strings.Join([]string{
			"Bash",
			"  Current shell:",
			"    source <(" + scriptCommand + ")",
			"  Persistent (Linux):",
			"    mkdir -p ~/.local/share/bash-completion/completions",
			"    " + scriptCommand + " > ~/.local/share/bash-completion/completions/go-doctor",
			"  Persistent (macOS/Homebrew bash-completion):",
			"    mkdir -p \"$(brew --prefix)/etc/bash_completion.d\"",
			"    " + scriptCommand + " > \"$(brew --prefix)/etc/bash_completion.d/go-doctor\"",
		}, "\n")
	case "zsh":
		return strings.Join([]string{
			"Zsh",
			"  Install:",
			"    mkdir -p ~/.zsh/completions",
			"    " + scriptCommand + " > ~/.zsh/completions/_go-doctor",
			"  Ensure ~/.zshrc loads that directory:",
			"    fpath=(~/.zsh/completions $fpath)",
			"    autoload -Uz compinit",
			"    compinit",
		}, "\n")
	case "fish":
		return strings.Join([]string{
			"Fish",
			"  Install:",
			"    mkdir -p ~/.config/fish/completions",
			"    " + scriptCommand + " > ~/.config/fish/completions/go-doctor.fish",
		}, "\n")
	case "powershell":
		return strings.Join([]string{
			"PowerShell",
			"  Current session:",
			"    " + scriptCommand + " | Out-String | Invoke-Expression",
			"  Persistent:",
			"    $dir = Join-Path $HOME \"Documents/PowerShell/Completions\"",
			"    New-Item -ItemType Directory -Force -Path $dir | Out-Null",
			"    $script = Join-Path $dir \"go-doctor.ps1\"",
			"    " + scriptCommand + " > $script",
			"    if (!(Test-Path $PROFILE)) { New-Item -ItemType File -Force -Path $PROFILE | Out-Null }",
			"    Add-Content $PROFILE \"`n. '$script'\"",
		}, "\n")
	default:
		return ""
	}
}

func renderRootHelp(cmd *cobra.Command, _ []string) {
	if cmd != cmd.Root() {
		renderSubcommandHelp(cmd)
		return
	}

	var builder strings.Builder

	builder.WriteString("go-doctor scans Go repositories for code health issues.\n\n")
	builder.WriteString("Usage:\n")
	builder.WriteString("  go-doctor [flags] [target]\n\n")
	builder.WriteString("Examples:\n")
	for _, line := range strings.Split(cmd.Example, "\n") {
		builder.WriteString("  ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteString("\n\n")

	for _, group := range rootFlagGroups {
		renderFlagGroup(&builder, cmd, group)
	}

	builder.WriteString("Commands:\n")
	builder.WriteString("  completion [shell]         Show completion install help.\n")
	builder.WriteString("  help [command]             Show help for a command.\n")

	_, _ = io.WriteString(cmd.OutOrStdout(), builder.String())
}

func renderSubcommandHelp(cmd *cobra.Command) {
	var builder strings.Builder

	if cmd.Long != "" {
		builder.WriteString(cmd.Long)
	} else {
		builder.WriteString(cmd.Short)
	}
	builder.WriteString("\n\n")
	builder.WriteString("Usage:\n")
	builder.WriteString("  ")
	builder.WriteString(cmd.UseLine())
	builder.WriteString("\n\n")

	if cmd.Example != "" {
		builder.WriteString("Examples:\n")
		for _, line := range strings.Split(cmd.Example, "\n") {
			builder.WriteString("  ")
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}

	if len(cmd.ValidArgs) > 0 {
		builder.WriteString("Valid Arguments:\n")
		builder.WriteString("  ")
		builder.WriteString(strings.Join(cmd.ValidArgs, ", "))
		builder.WriteString("\n\n")
	}

	flags := strings.TrimSpace(cmd.NonInheritedFlags().FlagUsagesWrapped(80))
	if flags != "" {
		builder.WriteString("Flags:\n")
		builder.WriteString(flags)
		builder.WriteString("\n\n")
	}

	commands := availableSubcommands(cmd)
	if len(commands) > 0 {
		builder.WriteString("Commands:\n")
		for _, subcommand := range commands {
			builder.WriteString("  ")
			builder.WriteString(fmt.Sprintf("%-28s %s", subcommand.UseLine(), subcommand.Short))
			builder.WriteByte('\n')
		}
		builder.WriteString("\n")
	}

	_, _ = io.WriteString(cmd.OutOrStdout(), builder.String())
}

func availableSubcommands(cmd *cobra.Command) []*cobra.Command {
	commands := make([]*cobra.Command, 0, len(cmd.Commands()))
	for _, subcommand := range cmd.Commands() {
		if !subcommand.IsAvailableCommand() || subcommand.IsAdditionalHelpTopicCommand() {
			continue
		}
		commands = append(commands, subcommand)
	}
	return commands
}

func renderFlagGroup(builder *strings.Builder, cmd *cobra.Command, group flagGroup) {
	var lines []string
	for _, name := range group.names {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			continue
		}
		lines = append(lines, formatFlagUsage(flag))
	}
	if len(lines) == 0 {
		return
	}

	builder.WriteString(group.title)
	builder.WriteByte('\n')
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteByte('\n')
}

func formatFlagUsage(flag *pflag.Flag) string {
	left := "--" + flag.Name
	if flag.Shorthand != "" {
		left = "-" + flag.Shorthand + ", " + left
	}
	flagType := flag.Value.Type()
	if flagType != "" && flagType != "bool" {
		left += " " + flagType
	}
	return fmt.Sprintf("  %-28s %s", left, flag.Usage)
}
