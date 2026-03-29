package formatter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

// FormatProgressFunc reports formatting progress (0.0 to 1.0) with a message.
type FormatProgressFunc func(progress float64, message string)

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
func GenerateMarkdown(meta *downloader.Metadata, segments []transcriber.Segment, cfg *config.TranscriptionConfig, onProgress FormatProgressFunc) (string, error) {
	var content strings.Builder
	totalSegments := len(segments)
	nextChapterIdx := 0

	if cfg.Timestamps {
		for i, seg := range segments {
			if onProgress != nil && totalSegments > 0 && i%50 == 0 {
				onProgress(float64(i)/float64(totalSegments)*0.6, "Building paragraphs...")
			}
			// Insert chapter heading if applicable
			if cfg.Chapters && len(meta.Chapters) > 0 {
				segSeconds := timestampToSeconds(seg.Start)
				for nextChapterIdx < len(meta.Chapters) &&
					segSeconds >= meta.Chapters[nextChapterIdx].StartTime {
					content.WriteString(fmt.Sprintf("### %s\n\n", meta.Chapters[nextChapterIdx].Title))
					nextChapterIdx++
				}
			}
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
			if onProgress != nil && totalSegments > 0 && i%50 == 0 {
				onProgress(float64(i)/float64(totalSegments)*0.6, "Building paragraphs...")
			}
			// Insert chapter heading if applicable
			if cfg.Chapters && len(meta.Chapters) > 0 {
				segSeconds := timestampToSeconds(seg.Start)
				for nextChapterIdx < len(meta.Chapters) &&
					segSeconds >= meta.Chapters[nextChapterIdx].StartTime {
					// Flush current paragraph before chapter heading
					if paragraph.Len() > 0 {
						text := strings.TrimSpace(paragraph.String())
						if text != "" {
							wrapped := wrapText(text, maxLineLength)
							content.WriteString(wrapped)
							content.WriteString("\n\n")
						}
						paragraph.Reset()
					}
					content.WriteString(fmt.Sprintf("### %s\n\n", meta.Chapters[nextChapterIdx].Title))
					nextChapterIdx++
				}
			}

			paragraph.WriteString(seg.Text)
			paragraph.WriteString(" ")

			var nextSeg *transcriber.Segment
			if i+1 < len(segments) {
				nextSeg = &segments[i+1]
			}
			if shouldBreakParagraph(paragraph.String(), seg, nextSeg) {
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

	filename := Slugify(meta.Title) + ".md"
	outputPath := filepath.Join(cfg.OutputDir, filename)

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	if onProgress != nil {
		onProgress(0.7, "Rendering template...")
	}

	tmpl, err := template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	if onProgress != nil {
		onProgress(0.8, "Fixing line wrapping...")
	}

	output := FixCommonIssues(buf.String(), onProgress)

	if onProgress != nil {
		onProgress(0.95, "Writing file...")
	}

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if onProgress != nil {
		onProgress(1.0, "Done")
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

// Slugify converts a string into a URL-safe slug for filenames.
func Slugify(s string) string {
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
func FixCommonIssues(content string, onProgress FormatProgressFunc) string {
	lines := strings.Split(content, "\n")
	fixedLines := make([]string, 0, len(lines))
	totalLines := len(lines)

	inFrontmatter := false
	for i, line := range lines {
		if onProgress != nil && totalLines > 0 && i%500 == 0 {
			// Map to 0.8-0.95 range
			onProgress(0.8+0.15*float64(i)/float64(totalLines), "Fixing line wrapping...")
		}
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
	content = collapseNewlines(content)
	content = strings.TrimRight(content, "\n") + "\n"

	return content
}

// collapseNewlines replaces runs of 3+ consecutive newlines with exactly 2,
// in a single pass (avoids the O(n^2) loop of repeated ReplaceAll).
func collapseNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	consecutive := 0
	for _, r := range s {
		if r == '\n' {
			consecutive++
			if consecutive <= 2 {
				b.WriteRune(r)
			}
		} else {
			consecutive = 0
			b.WriteRune(r)
		}
	}
	return b.String()
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

	words := markdownTokens(text)
	if len(words) == 0 {
		return []string{line}
	}

	var result []string
	var currentLine strings.Builder
	currentLine.WriteString(prefix)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		// Handle tokens longer than maxLen.
		// Markdown links must never be broken mid-link — output them as-is.
		// Plain long words are broken at character boundaries.
		if wordLen > maxLen {
			if lineLen > 0 {
				result = append(result, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(prefix)
				lineLen = 0
			}
			if markdownLinkRe.MatchString(word) {
				// Atomic markdown link — never break it, even if over limit.
				currentLine.WriteString(word)
				lineLen = wordLen
			} else {
				// Plain long word — break at character boundaries.
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
// Uses markdownTokens so that [text](url) links are never split across lines.
func wrapBlockquote(text string, maxLen int) string {
	// Account for "> " prefix (2 chars)
	effectiveLen := maxLen - 2

	if len(text) <= effectiveLen {
		return "> " + text
	}

	var result strings.Builder
	words := markdownTokens(text)
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

// shouldBreakParagraph decides whether to end the current paragraph after seg.
// It only breaks at sentence boundaries (ends with . ? !) and only when the
// paragraph has reached a reasonable length. A hard cap prevents run-on text.
func shouldBreakParagraph(paraText string, seg transcriber.Segment, nextSeg *transcriber.Segment) bool {
	text := strings.TrimSpace(seg.Text)
	endsWithPunct := strings.HasSuffix(text, ".") ||
		strings.HasSuffix(text, "?") ||
		strings.HasSuffix(text, "!")
	paraLen := len(strings.TrimSpace(paraText))

	if paraLen > 700 {
		return true // hard cap — never let a paragraph run indefinitely
	}
	if paraLen < 150 {
		return false // too short to end yet
	}
	if !endsWithPunct {
		return false // never break mid-sentence
	}
	// Only break if the next segment starts with a capital letter (new sentence/thought).
	if nextSeg != nil {
		nextText := strings.TrimSpace(nextSeg.Text)
		if len(nextText) > 0 && unicode.IsUpper([]rune(nextText)[0]) {
			return true
		}
		return false
	}
	return true // last segment
}

// markdownLinkRe matches inline Markdown links: [text](url)
var markdownLinkRe = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

// markdownTokens splits text into words while treating [text](url) links as
// single atomic tokens, so wrapping never breaks inside a Markdown link.
func markdownTokens(text string) []string {
	var tokens []string
	last := 0
	for _, loc := range markdownLinkRe.FindAllStringIndex(text, -1) {
		// words before the link
		tokens = append(tokens, strings.Fields(text[last:loc[0]])...)
		// the entire link as one token
		tokens = append(tokens, text[loc[0]:loc[1]])
		last = loc[1]
	}
	tokens = append(tokens, strings.Fields(text[last:])...)
	return tokens
}

// isWordBoundary checks if a rune is a word boundary character.
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r)
}

// timestampToSeconds converts a timestamp like "HH:MM:SS.mmm" or "MM:SS.mmm" to seconds.
func timestampToSeconds(ts string) float64 {
	ts = strings.ReplaceAll(ts, ",", ".")
	parts := strings.Split(ts, ":")
	switch len(parts) {
	case 3:
		h, _ := strconv.ParseFloat(parts[0], 64)
		m, _ := strconv.ParseFloat(parts[1], 64)
		s, _ := strconv.ParseFloat(parts[2], 64)
		return h*3600 + m*60 + s
	case 2:
		m, _ := strconv.ParseFloat(parts[0], 64)
		s, _ := strconv.ParseFloat(parts[1], 64)
		return m*60 + s
	default:
		s, _ := strconv.ParseFloat(ts, 64)
		return s
	}
}
