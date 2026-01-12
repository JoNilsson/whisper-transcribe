package formatter

import (
	"fmt"
	"os/exec"
	"strings"
)

// LintMarkdown validates the markdown file against markdownlint rules.
func LintMarkdown(path string) error {
	cmd := exec.Command("markdownlint", "--strict", path)
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
	cmd := exec.Command("markdownlint", path)
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
