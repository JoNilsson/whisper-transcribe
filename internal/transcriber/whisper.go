package transcriber

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cyber/whisper-transcribe/internal/models"
)

// Segment represents a transcribed segment with timestamps.
type Segment struct {
	Start     string
	End       string
	Text      string
	Timestamp string
}

// Chunk represents a streaming transcription chunk.
type Chunk struct {
	Text      string
	Timestamp string
	Progress  float64
}

// ChunkFunc is called for each transcription chunk.
type ChunkFunc func(chunk Chunk)

// Transcribe runs whisper.cpp on the audio file.
func Transcribe(ctx context.Context, audioPath string, model string, onChunk ChunkFunc) ([]Segment, error) {
	whisperBin := findWhisperBinary()
	if whisperBin == "" {
		return nil, fmt.Errorf("whisper binary not found in PATH (tried: whisper-cpp, whisper, main)")
	}

	modelPath := findModelPath(model)
	if modelPath == "" {
		return nil, fmt.Errorf("model '%s' not found - ensure whisper models are installed", model)
	}

	cmd := exec.CommandContext(ctx, whisperBin,
		"-m", modelPath,
		"-f", audioPath,
		"--output-txt",
		"--print-progress",
		"-pp",
		"-ml", "80",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start whisper: %w", err)
	}

	var segments []Segment
	progressRe := regexp.MustCompile(`progress\s*=\s*(\d+)`)
	timestampRe := regexp.MustCompile(`\[(\d{2}:\d{2}:\d{2}[.,]\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}[.,]\d{3})\]\s*(.*)`)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := progressRe.FindStringSubmatch(line); len(matches) > 1 {
				// Progress updates from stderr
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		if matches := timestampRe.FindStringSubmatch(line); len(matches) == 4 {
			seg := Segment{
				Start:     normalizeTimestamp(matches[1]),
				End:       normalizeTimestamp(matches[2]),
				Text:      strings.TrimSpace(matches[3]),
				Timestamp: formatTimestamp(matches[1]),
			}

			if seg.Text != "" {
				segments = append(segments, seg)

				if onChunk != nil {
					onChunk(Chunk{
						Text:      seg.Text,
						Timestamp: seg.Timestamp,
						Progress:  float64(lineCount) / 100.0,
					})
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if len(segments) > 0 {
			return segments, nil
		}
		return nil, fmt.Errorf("whisper failed: %w", err)
	}

	return segments, nil
}

func findWhisperBinary() string {
	names := []string{"whisper-cpp", "whisper", "main", "whisper-cli"}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	if bin := os.Getenv("WHISPER_BIN"); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}

	return ""
}

func findModelPath(model string) string {
	basePaths := []string{
		os.Getenv("WHISPER_MODEL_PATH"),
		models.GetModelsDir(),
		filepath.Join(os.Getenv("HOME"), ".whisper", "models"),
		filepath.Join(os.Getenv("HOME"), ".cache", "whisper"),
		"/usr/share/whisper/models",
		"/usr/local/share/whisper/models",
	}

	modelNames := []string{
		fmt.Sprintf("ggml-%s.bin", model),
		fmt.Sprintf("ggml-%s.en.bin", model),
		fmt.Sprintf("%s.bin", model),
		fmt.Sprintf("ggml-model-%s.bin", model),
	}

	for _, basePath := range basePaths {
		if basePath == "" {
			continue
		}
		for _, modelName := range modelNames {
			fullPath := filepath.Join(basePath, modelName)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}

	for _, modelName := range modelNames {
		if _, err := os.Stat(modelName); err == nil {
			return modelName
		}
	}

	return ""
}

func normalizeTimestamp(ts string) string {
	return strings.ReplaceAll(ts, ",", ".")
}

func formatTimestamp(ts string) string {
	ts = normalizeTimestamp(ts)
	parts := strings.Split(ts, ":")
	if len(parts) == 3 {
		h := parts[0]
		m := parts[1]
		s := strings.Split(parts[2], ".")[0]

		if h == "00" {
			return fmt.Sprintf("[%s:%s]", m, s)
		}
		return fmt.Sprintf("[%s:%s:%s]", h, m, s)
	}
	return "[" + ts + "]"
}

// CountWords counts words in segments.
func CountWords(segments []Segment) int {
	count := 0
	for _, seg := range segments {
		words := strings.Fields(seg.Text)
		count += len(words)
	}
	return count
}

// ModelExists checks if a whisper model is available locally.
func ModelExists(model string) bool {
	return findModelPath(model) != ""
}

// ErrModelNotFound is returned when a model is not found locally.
type ErrModelNotFound struct {
	Model string
}

func (e ErrModelNotFound) Error() string {
	return fmt.Sprintf("model '%s' not found locally", e.Model)
}

// CheckModel verifies the model exists, returning ErrModelNotFound if not.
func CheckModel(model string) error {
	if !ModelExists(model) {
		return ErrModelNotFound{Model: model}
	}
	return nil
}
