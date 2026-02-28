package thirdparty

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type execResult struct {
	stdout   string
	stderr   string
	exitCode int
}

type execRunner func(ctx context.Context, dir string, name string, args ...string) (execResult, error)

var runCommand execRunner = defaultRunCommand

func defaultRunCommand(ctx context.Context, dir string, name string, args ...string) (execResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := execResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func missingToolError(tool string, installHint string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%s is not installed or not on PATH; install with: %s", tool, installHint)
	}
	return err
}

func combinedOutput(result execResult) string {
	text := strings.TrimSpace(result.stdout)
	if text != "" {
		return text
	}
	return strings.TrimSpace(result.stderr)
}
