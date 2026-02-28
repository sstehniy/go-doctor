package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra/doc"
	"github.com/sstehniy/go-doctor/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var check bool

	flags := flag.NewFlagSet("generate-cli-assets", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&check, "check", false, "verify generated assets without writing files")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	generatedDir, err := os.MkdirTemp("", "go-doctor-cli-assets-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(generatedDir)

	if err := generateAssets(generatedDir); err != nil {
		return err
	}

	targetDir := filepath.Join(repoRoot, "docs", "cli")
	if check {
		if err := compareTrees(generatedDir, targetDir); err != nil {
			return fmt.Errorf("%w\nRun: go run ./scripts/generate-cli-assets.go", err)
		}
		return nil
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("reset %q: %w", targetDir, err)
	}
	if err := copyTree(generatedDir, targetDir); err != nil {
		return err
	}

	return nil
}

func generateAssets(root string) error {
	command := app.NewRootCommand(context.Background(), io.Discard, io.Discard)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create docs dir: %w", err)
	}
	if err := doc.GenMarkdownTree(command, root); err != nil {
		return fmt.Errorf("generate markdown docs: %w", err)
	}

	completionDir := filepath.Join(root, "completions")
	if err := os.MkdirAll(completionDir, 0o755); err != nil {
		return fmt.Errorf("create completion dir: %w", err)
	}

	generators := []struct {
		filename string
		write    func(io.Writer) error
	}{
		{
			filename: "go-doctor.bash",
			write: func(writer io.Writer) error {
				return command.GenBashCompletionV2(writer, true)
			},
		},
		{
			filename: "go-doctor.zsh",
			write: func(writer io.Writer) error {
				return command.GenZshCompletion(writer)
			},
		},
		{
			filename: "go-doctor.fish",
			write: func(writer io.Writer) error {
				return command.GenFishCompletion(writer, true)
			},
		},
		{
			filename: "go-doctor.ps1",
			write: func(writer io.Writer) error {
				return command.GenPowerShellCompletionWithDesc(writer)
			},
		},
	}

	for _, generator := range generators {
		path := filepath.Join(completionDir, generator.filename)
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create %q: %w", path, err)
		}
		if err := generator.write(file); err != nil {
			file.Close()
			return fmt.Errorf("generate %q: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close %q: %w", path, err)
		}
	}

	return nil
}

func compareTrees(wantRoot string, gotRoot string) error {
	wantFiles, err := collectFiles(wantRoot)
	if err != nil {
		return err
	}
	gotFiles, err := collectFiles(gotRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("generated assets are missing from %q", gotRoot)
		}
		return err
	}

	wantNames := sortedKeys(wantFiles)
	gotNames := sortedKeys(gotFiles)
	if len(wantNames) != len(gotNames) {
		return fmt.Errorf("generated asset file count differs: want %d, got %d", len(wantNames), len(gotNames))
	}

	for index := range wantNames {
		if wantNames[index] != gotNames[index] {
			return fmt.Errorf("generated asset set differs: want %q, got %q", wantNames[index], gotNames[index])
		}
	}

	for _, name := range wantNames {
		if !bytes.Equal(wantFiles[name], gotFiles[name]) {
			return fmt.Errorf("generated asset differs: %s", filepath.ToSlash(name))
		}
	}

	return nil
}

func collectFiles(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func sortedKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyTree(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
