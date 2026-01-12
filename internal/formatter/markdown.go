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

	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

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

> Transcribed from [{{.Channel}}]({{.ChannelURL}}) on {{.TranscribedDate}}

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
	Content         string
}

// GenerateMarkdown creates a Markdown file from transcription segments.
func GenerateMarkdown(meta *downloader.Metadata, segments []transcriber.Segment, cfg *config.TranscriptionConfig) (string, error) {
	var content strings.Builder

	if cfg.Timestamps {
		for _, seg := range segments {
			content.WriteString(fmt.Sprintf("**%s** %s\n\n", seg.Timestamp, seg.Text))
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
					content.WriteString(text)
					content.WriteString("\n\n")
				}
				paragraph.Reset()
			}
		}
		if paragraph.Len() > 0 {
			text := strings.TrimSpace(paragraph.String())
			if text != "" {
				content.WriteString(text)
				content.WriteString("\n")
			}
		}
	}

	uploadDate := meta.UploadDate
	if len(uploadDate) == 8 {
		uploadDate = fmt.Sprintf("%s-%s-%s", uploadDate[:4], uploadDate[4:6], uploadDate[6:8])
	}

	data := MarkdownData{
		Title:           sanitizeTitle(meta.Title),
		Source:          cfg.URL,
		Channel:         meta.Channel,
		ChannelURL:      meta.ChannelURL,
		UploadDate:      uploadDate,
		TranscribedDate: time.Now().Format("2006-01-02"),
		Duration:        meta.Duration,
		Model:           cfg.Model,
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
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	content = strings.Join(lines, "\n")

	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	content = strings.TrimRight(content, "\n") + "\n"

	return content
}
