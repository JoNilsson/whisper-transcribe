package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Metadata holds video information from YouTube.
type Metadata struct {
	Title       string
	Channel     string
	ChannelURL  string
	Duration    string
	DurationSec int
	UploadDate  string
	Description string
	VideoID     string
}

// ProgressFunc is called with download progress (0.0 to 1.0).
type ProgressFunc func(progress float64)

// FetchMetadata retrieves video information without downloading.
func FetchMetadata(ctx context.Context, url string) (*Metadata, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--dump-json",
		"--no-download",
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp metadata failed: %w", err)
	}

	var data struct {
		Title       string `json:"title"`
		Channel     string `json:"channel"`
		ChannelURL  string `json:"channel_url"`
		Duration    int    `json:"duration"`
		UploadDate  string `json:"upload_date"`
		Description string `json:"description"`
		ID          string `json:"id"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	return &Metadata{
		Title:       data.Title,
		Channel:     data.Channel,
		ChannelURL:  data.ChannelURL,
		Duration:    formatDuration(data.Duration),
		DurationSec: data.Duration,
		UploadDate:  data.UploadDate,
		Description: data.Description,
		VideoID:     data.ID,
	}, nil
}

// Download extracts audio from the video and converts to WAV format.
func Download(ctx context.Context, url string, onProgress ProgressFunc) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "whisper-transcribe")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	outputTemplate := filepath.Join(tmpDir, "%(id)s.%(ext)s")

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--extract-audio",
		"--audio-format", "wav",
		"--audio-quality", "0",
		"--postprocessor-args", "ffmpeg:-ar 16000 -ac 1",
		"--newline",
		"--progress",
		"-o", outputTemplate,
		url,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start yt-dlp: %w", err)
	}

	progressRe := regexp.MustCompile(`(\d+\.?\d*)%`)
	var audioPath string

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Consume stderr to prevent blocking
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if matches := progressRe.FindStringSubmatch(line); len(matches) > 1 {
			if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
				if onProgress != nil {
					onProgress(pct / 100.0)
				}
			}
		}

		if strings.Contains(line, "[ExtractAudio] Destination:") {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				audioPath = strings.TrimSpace(parts[1])
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	if audioPath == "" {
		files, _ := filepath.Glob(filepath.Join(tmpDir, "*.wav"))
		if len(files) > 0 {
			audioPath = files[len(files)-1]
		} else {
			return "", fmt.Errorf("no audio file produced")
		}
	}

	return audioPath, nil
}

// ValidateURL checks if the URL is a valid YouTube URL.
func ValidateURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("URL is required")
	}
	if !strings.Contains(url, "youtube.com/watch") &&
		!strings.Contains(url, "youtu.be/") &&
		!strings.Contains(url, "youtube.com/shorts/") {
		return fmt.Errorf("invalid YouTube URL")
	}
	return nil
}

func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
