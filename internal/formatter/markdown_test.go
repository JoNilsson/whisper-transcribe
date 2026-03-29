package formatter

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   int // max line length in output
	}{
		{
			name:   "short text unchanged",
			input:  "Hello world",
			maxLen: 80,
			want:   80,
		},
		{
			name:   "long text wrapped",
			input:  "This is a very long sentence that should be wrapped at the specified maximum line length to comply with markdown lint rules.",
			maxLen: 80,
			want:   80,
		},
		{
			name:   "multiple sentences",
			input:  "First sentence here. Second sentence that is longer. Third sentence to make this text exceed the line limit significantly.",
			maxLen: 40,
			want:   40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.input, tt.maxLen)
			for i, line := range splitLines(result) {
				if len(line) > tt.want {
					t.Errorf("line %d exceeds max length: got %d, want <= %d\nline: %q",
						i, len(line), tt.want, line)
				}
			}
		})
	}
}

func TestWrapBlockquote(t *testing.T) {
	t.Run("short blockquote stays on one line", func(t *testing.T) {
		result := wrapBlockquote("Short quote", 80)
		if result != "> Short quote" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("long plain text wraps within limit", func(t *testing.T) {
		input := "This is a very long blockquote line that definitely exceeds the eighty character limit and should be wrapped properly."
		result := wrapBlockquote(input, 80)
		for i, line := range splitLines(result) {
			if len(line) > 80 {
				t.Errorf("line %d exceeds 80 chars: %q", i, line)
			}
			if line != "" && !hasBlockquotePrefix(line) {
				t.Errorf("line %d missing blockquote prefix: %q", i, line)
			}
		}
	})

	t.Run("markdown link is never split across lines", func(t *testing.T) {
		// Link token itself is longer than 78 chars — must stay atomic.
		input := "Transcribed from [Very Long Channel Name Here](https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxxxx) on 2024-01-15"
		result := wrapBlockquote(input, 80)
		for i, line := range splitLines(result) {
			if line != "" && !hasBlockquotePrefix(line) {
				t.Errorf("line %d missing blockquote prefix: %q", i, line)
			}
			// A line with an open bracket must also have the closing ](
			if strings.Contains(line, "[") && !strings.Contains(line, "](") {
				t.Errorf("line %d has a broken markdown link: %q", i, line)
			}
		}
	})
}

func TestGenerateMarkdownLintCompliant(t *testing.T) {
	// Skip if markdownlint is not available
	if _, err := lookupMarkdownlint(); err != nil {
		t.Skip("markdownlint not available")
	}

	tmpDir := t.TempDir()

	meta := &downloader.Metadata{
		Title:      "Test Video Title",
		Channel:    "Test Channel",
		ChannelURL: "https://www.youtube.com/channel/test",
		Duration:   "10:30",
		UploadDate: "20240115",
	}

	segments := []transcriber.Segment{
		{Text: "This is the first segment of transcribed text.", Timestamp: "00:00"},
		{Text: "Here is another segment with more content.", Timestamp: "00:05"},
		{Text: "And a third segment to test paragraph formation.", Timestamp: "00:10"},
		{Text: "Final segment with ending punctuation.", Timestamp: "00:15"},
	}

	cfg := &config.TranscriptionConfig{
		URL:        "https://www.youtube.com/watch?v=test123",
		Model:      "base",
		Timestamps: false,
		OutputDir:  tmpDir,
	}

	outputPath, err := GenerateMarkdown(meta, segments, cfg, nil)
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file not created: %s", outputPath)
	}

	// Run markdownlint
	if err := LintMarkdown(outputPath); err != nil {
		content, _ := os.ReadFile(outputPath)
		t.Errorf("Markdown lint failed: %v\n\nContent:\n%s", err, string(content))
	}
}

func TestGenerateMarkdownWithTimestamps(t *testing.T) {
	// Skip if markdownlint is not available
	if _, err := lookupMarkdownlint(); err != nil {
		t.Skip("markdownlint not available")
	}

	tmpDir := t.TempDir()

	meta := &downloader.Metadata{
		Title:      "Test Video With Timestamps",
		Channel:    "Test Channel",
		ChannelURL: "https://www.youtube.com/channel/test",
		Duration:   "5:00",
		UploadDate: "20240115",
	}

	segments := []transcriber.Segment{
		{Text: "First segment text here.", Timestamp: "00:00"},
		{Text: "Second segment with longer text that might need wrapping if it gets too long.", Timestamp: "00:30"},
	}

	cfg := &config.TranscriptionConfig{
		URL:        "https://www.youtube.com/watch?v=test456",
		Model:      "small",
		Timestamps: true,
		OutputDir:  tmpDir,
	}

	outputPath, err := GenerateMarkdown(meta, segments, cfg, nil)
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	if err := LintMarkdown(outputPath); err != nil {
		content, _ := os.ReadFile(outputPath)
		t.Errorf("Markdown lint failed: %v\n\nContent:\n%s", err, string(content))
	}
}

func TestGenerateMarkdownLongContent(t *testing.T) {
	// Skip if markdownlint is not available
	if _, err := lookupMarkdownlint(); err != nil {
		t.Skip("markdownlint not available")
	}

	tmpDir := t.TempDir()

	meta := &downloader.Metadata{
		Title:      "Video With Very Long Content Lines That Need Wrapping",
		Channel:    "A Channel With A Really Long Name That Might Cause Issues",
		ChannelURL: "https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Duration:   "1:00:00",
		UploadDate: "20240115",
	}

	// Create segments with very long text
	segments := []transcriber.Segment{
		{
			Text:      "This is an extremely long segment of transcribed text that definitely exceeds the eighty character line limit and needs to be properly wrapped to comply with markdown lint rules for line length.",
			Timestamp: "00:00",
		},
		{
			Text:      "Another very long segment here with lots of words that will need to be wrapped properly across multiple lines to ensure compliance.",
			Timestamp: "01:00",
		},
	}

	cfg := &config.TranscriptionConfig{
		URL:        "https://www.youtube.com/watch?v=longvideo123",
		Model:      "medium",
		Timestamps: false,
		OutputDir:  tmpDir,
	}

	outputPath, err := GenerateMarkdown(meta, segments, cfg, nil)
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	// Verify all lines are within limit
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	inFrontmatter := false
	for i, line := range splitLines(string(content)) {
		// Track frontmatter (YAML allows longer lines)
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}
		// Lines containing markdown links may exceed 80 chars — we never break
		// a link mid-syntax to preserve link integrity.
		if strings.Contains(line, "](") {
			continue
		}
		if len(line) > 80 {
			t.Errorf("Line %d exceeds 80 chars (len=%d): %q", i+1, len(line), line)
		}
	}

	if err := LintMarkdown(outputPath); err != nil {
		t.Errorf("Markdown lint failed: %v", err)
	}
}

func TestMarkdownTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "plain words",
			input: "hello world foo",
			want:  []string{"hello", "world", "foo"},
		},
		{
			name:  "link is single token",
			input: "see [Bernie Sanders](https://example.com) here",
			want:  []string{"see", "[Bernie Sanders](https://example.com)", "here"},
		},
		{
			name:  "multiple links",
			input: "[A](http://a.com) and [B](http://b.com)",
			want:  []string{"[A](http://a.com)", "and", "[B](http://b.com)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownTokens(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWrapBlockquotePreservesLinks(t *testing.T) {
	// Attribution line with a markdown link — the link must never be split.
	input := "Transcribed from [Bernie Sanders](https://www.youtube.com/channel/UCH1dpzjCEiGAt8CXkryhkZg) on 2026-03-29"
	result := wrapBlockquote(input, 80)
	for i, line := range splitLines(result) {
		// Every line should start with "> "
		if line != "" && !hasBlockquotePrefix(line) {
			t.Errorf("line %d missing blockquote prefix: %q", i, line)
		}
		// No line should contain a broken link (open bracket without closing paren on the same line)
		if strings.Contains(line, "[") && !strings.Contains(line, "](") {
			t.Errorf("line %d contains a broken markdown link: %q", i, line)
		}
	}
}

func TestShouldBreakParagraph(t *testing.T) {
	makeSeg := func(text string) transcriber.Segment { return transcriber.Segment{Text: text} }
	makeNext := func(text string) *transcriber.Segment { s := makeSeg(text); return &s }

	tests := []struct {
		name      string
		paraText  string
		seg       transcriber.Segment
		nextSeg   *transcriber.Segment
		wantBreak bool
	}{
		{
			name:      "short paragraph, no break even with punctuation",
			paraText:  "Short text.",
			seg:       makeSeg("Short text."),
			nextSeg:   makeNext("Next sentence."),
			wantBreak: false,
		},
		{
			name:      "long paragraph, no punctuation, no break",
			paraText:  strings.Repeat("word ", 50),
			seg:       makeSeg("continuation without punctuation"),
			nextSeg:   makeNext("More text here."),
			wantBreak: false,
		},
		{
			name:      "long paragraph, ends with period, next starts capital",
			paraText:  strings.Repeat("word ", 50),
			seg:       makeSeg("Ends with a period."),
			nextSeg:   makeNext("Capital start here."),
			wantBreak: true,
		},
		{
			name:      "long paragraph, ends with period, next starts lowercase",
			paraText:  strings.Repeat("word ", 50),
			seg:       makeSeg("Ends with a period."),
			nextSeg:   makeNext("lowercase continuation here."),
			wantBreak: false,
		},
		{
			name:      "hard cap at 700 chars",
			paraText:  strings.Repeat("x", 701),
			seg:       makeSeg("no punctuation here"),
			nextSeg:   makeNext("next"),
			wantBreak: true,
		},
		{
			name:      "last segment breaks after sentence",
			paraText:  strings.Repeat("word ", 50),
			seg:       makeSeg("Final sentence here."),
			nextSeg:   nil,
			wantBreak: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBreakParagraph(tt.paraText, tt.seg, tt.nextSeg)
			if got != tt.wantBreak {
				t.Errorf("shouldBreakParagraph() = %v, want %v", got, tt.wantBreak)
			}
		})
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func hasBlockquotePrefix(line string) bool {
	return len(line) >= 2 && line[0] == '>' && (line[1] == ' ' || line[1] == '\n')
}

func lookupMarkdownlint() (string, error) {
	return exec.LookPath("markdownlint")
}
