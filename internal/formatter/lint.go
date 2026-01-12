package formatter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// markdownlintConfig is the embedded configuration for markdownlint.
const markdownlintConfig = `{
  "default": true,
  "MD013": {
    "line_length": 80,
    "code_blocks": false,
    "tables": false
  },
  "MD025": {
    "front_matter_title": ""
  },
  "MD041": false
}`

// LintMarkdown validates the markdown file against markdownlint rules.
func LintMarkdown(path string) error {
	// Create a temporary config file
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "whisper-markdownlint.json")

	if err := os.WriteFile(configPath, []byte(markdownlintConfig), 0644); err != nil {
		return fmt.Errorf("create lint config: %w", err)
	}
	defer os.Remove(configPath)

	cmd := exec.Command("markdownlint", "--config", configPath, path)
	output, err := cmd.CombinedOutput()

	if err != nil {
		violations := strings.TrimSpace(string(output))
		if violations != "" {
			return fmt.Errorf("markdown lint failed:\n%s", violations)
		}
		return fmt.Errorf("markdownlint error: %w", err)
	}

	return nil
}

// LintMarkdownSoft validates but returns warnings instead of errors.
func LintMarkdownSoft(path string) ([]string, error) {
	// Create a temporary config file
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "whisper-markdownlint.json")

	if err := os.WriteFile(configPath, []byte(markdownlintConfig), 0644); err != nil {
		return nil, fmt.Errorf("create lint config: %w", err)
	}
	defer os.Remove(configPath)

	cmd := exec.Command("markdownlint", "--config", configPath, path)
	output, _ := cmd.CombinedOutput()

	violations := strings.TrimSpace(string(output))
	if violations == "" {
		return nil, nil
	}

	lines := strings.Split(violations, "\n")
	var warnings []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			warnings = append(warnings, line)
		}
	}

	return warnings, nil
}
