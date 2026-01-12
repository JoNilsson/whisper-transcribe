package formatter

import (
	"os"
	"path/filepath"
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
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{
			name:   "short blockquote",
			input:  "Short quote",
			maxLen: 80,
		},
		{
			name:   "long blockquote with link",
			input:  "Transcribed from [Very Long Channel Name Here](https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxxxx) on 2024-01-15",
			maxLen: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapBlockquote(tt.input, tt.maxLen)
			for i, line := range splitLines(result) {
				if len(line) > tt.maxLen {
					t.Errorf("line %d exceeds max length: got %d, want <= %d\nline: %q",
						i, len(line), tt.maxLen, line)
				}
				if line != "" && !hasBlockquotePrefix(line) {
					t.Errorf("line %d missing blockquote prefix: %q", i, line)
				}
			}
		})
	}
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

	outputPath, err := GenerateMarkdown(meta, segments, cfg)
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

	outputPath, err := GenerateMarkdown(meta, segments, cfg)
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

	outputPath, err := GenerateMarkdown(meta, segments, cfg)
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
		if len(line) > 80 {
			t.Errorf("Line %d exceeds 80 chars (len=%d): %q", i+1, len(line), line)
		}
	}

	if err := LintMarkdown(outputPath); err != nil {
		t.Errorf("Markdown lint failed: %v", err)
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
	return filepath.Abs("markdownlint")
}
