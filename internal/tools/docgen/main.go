// Command docgen generates markdown CLI documentation from cobra commands.
// Output goes to docs/cli/ for LLM consumption and llms.txt aggregation.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cmd "github.com/hironow/phonewave/internal/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	outDir := filepath.Join("docs", "cli")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCommand()
	rootCmd.DisableAutoGenTag = true

	if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "docgen: %v\n", err)
		os.Exit(1)
	}
	if err := normalizeGeneratedMarkdown(outDir); err != nil {
		fmt.Fprintf(os.Stderr, "normalize docs: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "docs generated in %s/\n", outDir)
}

func normalizeGeneratedMarkdown(outDir string) error {
	return filepath.WalkDir(outDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		normalized := normalizeMarkdown(string(data))
		return os.WriteFile(path, []byte(normalized), 0o644)
	})
}

func normalizeMarkdown(content string) string {
	normalized := strings.ReplaceAll(content, "\t - ", "  - ")
	return strings.TrimRight(normalized, "\n") + "\n"
}
