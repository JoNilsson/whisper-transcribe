package formatter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

const maxLineLength = 80

const markdownTemplate = `---
title: "{{.Title}}"
source: "{{.Source}}"
channel: "{{.Channel}}"
uploaded: "{{.UploadDate}}"
transcribed: "{{.TranscribedDate}}"
duration: "{{.Duration}}"
model: "whisper-{{.Model}}"
---

# {{.Title}}

{{.Attribution}}

## Transcription

{{.Content}}
`

// MarkdownData holds data for template rendering.
type MarkdownData struct {
	Title           string
	Source          string
	Channel         string
	ChannelURL      string
	UploadDate      string
	TranscribedDate string
	Duration        string
	Model           string
	Attribution     string
	Content         string
}

// GenerateMarkdown creates a Markdown file from transcription segments.
func GenerateMarkdown(meta *downloader.Metadata, segments []transcriber.Segment, cfg *config.TranscriptionConfig) (string, error) {
	var content strings.Builder

	if cfg.Timestamps {
		for _, seg := range segments {
			// Format: **[00:00]** Text wrapped to 80 chars
			timestamp := fmt.Sprintf("**[%s]**", seg.Timestamp)
			text := strings.TrimSpace(seg.Text)
			// Wrap text accounting for timestamp prefix on first line
			wrapped := wrapTextWithPrefix(timestamp+" ", text, maxLineLength)
			content.WriteString(wrapped)
			content.WriteString("\n\n")
		}
	} else {
		var paragraph strings.Builder
		for i, seg := range segments {
			paragraph.WriteString(seg.Text)
			paragraph.WriteString(" ")

			if strings.HasSuffix(seg.Text, ".") ||
				strings.HasSuffix(seg.Text, "?") ||
				strings.HasSuffix(seg.Text, "!") ||
				(i+1)%5 == 0 {
				text := strings.TrimSpace(paragraph.String())
				if text != "" {
					wrapped := wrapText(text, maxLineLength)
					content.WriteString(wrapped)
					content.WriteString("\n\n")
				}
				paragraph.Reset()
			}
		}
		if paragraph.Len() > 0 {
			text := strings.TrimSpace(paragraph.String())
			if text != "" {
				wrapped := wrapText(text, maxLineLength)
				content.WriteString(wrapped)
				content.WriteString("\n")
			}
		}
	}

	uploadDate := meta.UploadDate
	if len(uploadDate) == 8 {
		uploadDate = fmt.Sprintf("%s-%s-%s", uploadDate[:4], uploadDate[4:6], uploadDate[6:8])
	}

	transcribedDate := time.Now().Format("2006-01-02")

	// Build attribution line, wrapped if needed
	var attribution string
	if meta.ChannelURL != "" {
		attribution = fmt.Sprintf(
			"Transcribed from [%s](%s) on %s",
			meta.Channel, meta.ChannelURL, transcribedDate,
		)
	} else {
		attribution = fmt.Sprintf(
			"Transcribed from %s on %s",
			meta.Channel, transcribedDate,
		)
	}
	// Wrap attribution as blockquote (accounting for "> " prefix)
	attribution = wrapBlockquote(attribution, maxLineLength)

	data := MarkdownData{
		Title:           sanitizeTitle(meta.Title),
		Source:          cfg.GetSource(),
		Channel:         meta.Channel,
		ChannelURL:      meta.ChannelURL,
		UploadDate:      uploadDate,
		TranscribedDate: transcribedDate,
		Duration:        meta.Duration,
		Model:           cfg.Model,
		Attribution:     attribution,
		Content:         strings.TrimSpace(content.String()),
	}

	filename := slugify(meta.Title) + ".md"
	outputPath := filepath.Join(cfg.OutputDir, filename)

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	tmpl, err := template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	output := FixCommonIssues(buf.String())

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return outputPath, nil
}

func sanitizeTitle(title string) string {
	title = strings.ReplaceAll(title, `"`, `'`)
	title = strings.ReplaceAll(title, `:`, "-")
	title = strings.ReplaceAll(title, `\`, "-")
	title = strings.ReplaceAll(title, `/`, "-")
	return strings.TrimSpace(title)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		lastDash := strings.LastIndex(s, "-")
		if lastDash > 40 {
			s = s[:lastDash]
		}
	}
	return s
}

// FixCommonIssues applies automatic fixes for common lint violations.
func FixCommonIssues(content string) string {
	lines := strings.Split(content, "\n")
	var fixedLines []string

	inFrontmatter := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")

		// Track frontmatter (don't wrap YAML)
		if line == "---" {
			inFrontmatter = !inFrontmatter
			fixedLines = append(fixedLines, line)
			continue
		}

		// Don't wrap frontmatter or short lines
		if inFrontmatter || len(line) <= maxLineLength {
			fixedLines = append(fixedLines, line)
			continue
		}

		// Force wrap long lines
		wrapped := forceWrapLine(line, maxLineLength)
		fixedLines = append(fixedLines, wrapped...)
	}

	content = strings.Join(fixedLines, "\n")

	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	content = strings.TrimRight(content, "\n") + "\n"

	return content
}

// forceWrapLine wraps a single line that exceeds maxLen.
func forceWrapLine(line string, maxLen int) []string {
	// Check for blockquote prefix
	prefix := ""
	text := line
	if strings.HasPrefix(line, "> ") {
		prefix = "> "
		text = line[2:]
		maxLen -= 2
	}

	// Check for heading (don't wrap headings, just truncate if needed)
	if strings.HasPrefix(line, "#") {
		return []string{line}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{line}
	}

	var result []string
	var currentLine strings.Builder
	currentLine.WriteString(prefix)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		// Handle words longer than maxLen by breaking them
		if wordLen > maxLen {
			if lineLen > 0 {
				result = append(result, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(prefix)
				lineLen = 0
			}
			// Break the long word
			for len(word) > maxLen {
				currentLine.WriteString(word[:maxLen])
				result = append(result, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(prefix)
				word = word[maxLen:]
			}
			if len(word) > 0 {
				currentLine.WriteString(word)
				lineLen = len(word)
			}
			continue
		}

		if i == 0 {
			currentLine.WriteString(word)
			lineLen = wordLen
			continue
		}

		if lineLen+1+wordLen > maxLen {
			result = append(result, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(prefix)
			currentLine.WriteString(word)
			lineLen = wordLen
		} else {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
			lineLen += 1 + wordLen
		}
	}

	if currentLine.Len() > len(prefix) {
		result = append(result, currentLine.String())
	}

	return result
}

// wrapText wraps text to the specified line length, breaking at word boundaries.
func wrapText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if i == 0 {
			result.WriteString(word)
			lineLen = wordLen
			continue
		}

		if lineLen+1+wordLen > maxLen {
			result.WriteString("\n")
			result.WriteString(word)
			lineLen = wordLen
		} else {
			result.WriteString(" ")
			result.WriteString(word)
			lineLen += 1 + wordLen
		}
	}

	return result.String()
}

// wrapTextWithPrefix wraps text with a prefix on the first line.
func wrapTextWithPrefix(prefix, text string, maxLen int) string {
	if len(prefix)+len(text) <= maxLen {
		return prefix + text
	}

	var result strings.Builder
	words := strings.Fields(text)
	result.WriteString(prefix)
	lineLen := len(prefix)

	for i, word := range words {
		wordLen := len(word)

		if i == 0 {
			result.WriteString(word)
			lineLen += wordLen
			continue
		}

		if lineLen+1+wordLen > maxLen {
			result.WriteString("\n")
			result.WriteString(word)
			lineLen = wordLen
		} else {
			result.WriteString(" ")
			result.WriteString(word)
			lineLen += 1 + wordLen
		}
	}

	return result.String()
}

// wrapBlockquote wraps text as a markdown blockquote.
func wrapBlockquote(text string, maxLen int) string {
	// Account for "> " prefix (2 chars)
	effectiveLen := maxLen - 2

	if len(text) <= effectiveLen {
		return "> " + text
	}

	var result strings.Builder
	words := strings.Fields(text)
	result.WriteString("> ")
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if i == 0 {
			result.WriteString(word)
			lineLen = wordLen
			continue
		}

		if lineLen+1+wordLen > effectiveLen {
			result.WriteString("\n> ")
			result.WriteString(word)
			lineLen = wordLen
		} else {
			result.WriteString(" ")
			result.WriteString(word)
			lineLen += 1 + wordLen
		}
	}

	return result.String()
}

// isWordBoundary checks if a rune is a word boundary character.
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r)
}
