package models

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// HuggingFaceBaseURL is the base URL for whisper.cpp models on Hugging Face.
	HuggingFaceBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"
)

// ModelInfo contains information about a whisper model.
type ModelInfo struct {
	Name     string
	Filename string
	Size     string
	URL      string
}

// ProgressFunc is called with download progress (bytesDownloaded, totalBytes).
type ProgressFunc func(downloaded, total int64)

// AvailableModels returns information about all available models.
func AvailableModels() []ModelInfo {
	return []ModelInfo{
		{Name: "tiny", Filename: "ggml-tiny.bin", Size: "75 MB", URL: HuggingFaceBaseURL + "/ggml-tiny.bin"},
		{Name: "tiny.en", Filename: "ggml-tiny.en.bin", Size: "75 MB", URL: HuggingFaceBaseURL + "/ggml-tiny.en.bin"},
		{Name: "base", Filename: "ggml-base.bin", Size: "142 MB", URL: HuggingFaceBaseURL + "/ggml-base.bin"},
		{Name: "base.en", Filename: "ggml-base.en.bin", Size: "142 MB", URL: HuggingFaceBaseURL + "/ggml-base.en.bin"},
		{Name: "small", Filename: "ggml-small.bin", Size: "466 MB", URL: HuggingFaceBaseURL + "/ggml-small.bin"},
		{Name: "small.en", Filename: "ggml-small.en.bin", Size: "466 MB", URL: HuggingFaceBaseURL + "/ggml-small.en.bin"},
		{Name: "medium", Filename: "ggml-medium.bin", Size: "1.5 GB", URL: HuggingFaceBaseURL + "/ggml-medium.bin"},
		{Name: "medium.en", Filename: "ggml-medium.en.bin", Size: "1.5 GB", URL: HuggingFaceBaseURL + "/ggml-medium.en.bin"},
		{Name: "large-v1", Filename: "ggml-large-v1.bin", Size: "2.9 GB", URL: HuggingFaceBaseURL + "/ggml-large-v1.bin"},
		{Name: "large-v2", Filename: "ggml-large-v2.bin", Size: "2.9 GB", URL: HuggingFaceBaseURL + "/ggml-large-v2.bin"},
		{Name: "large-v3", Filename: "ggml-large-v3.bin", Size: "2.9 GB", URL: HuggingFaceBaseURL + "/ggml-large-v3.bin"},
		{Name: "large", Filename: "ggml-large-v3.bin", Size: "2.9 GB", URL: HuggingFaceBaseURL + "/ggml-large-v3.bin"},
	}
}

// GetModelInfo returns information about a specific model.
func GetModelInfo(name string) (*ModelInfo, error) {
	for _, m := range AvailableModels() {
		if m.Name == name {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("unknown model: %s", name)
}

// GetModelsDir returns the directory where models are stored.
func GetModelsDir() string {
	// Check environment variable first
	if dir := os.Getenv("WHISPER_MODEL_PATH"); dir != "" {
		return dir
	}

	// Default to ~/.cache/whisper
	home, err := os.UserHomeDir()
	if err != nil {
		return "./models"
	}
	return filepath.Join(home, ".cache", "whisper")
}

// GetModelPath returns the full path to a model file.
func GetModelPath(name string) (string, error) {
	info, err := GetModelInfo(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(GetModelsDir(), info.Filename), nil
}

// ModelExists checks if a model is already downloaded.
func ModelExists(name string) bool {
	path, err := GetModelPath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Download downloads a model from Hugging Face.
func Download(name string, onProgress ProgressFunc) error {
	info, err := GetModelInfo(name)
	if err != nil {
		return err
	}

	modelsDir := GetModelsDir()
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("create models directory: %w", err)
	}

	destPath := filepath.Join(modelsDir, info.Filename)
	tmpPath := destPath + ".tmp"

	// Create temporary file
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		out.Close()
		os.Remove(tmpPath) // Clean up on error
	}()

	// Start download
	resp, err := http.Get(info.URL)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength

	// Create progress reader
	reader := &progressReader{
		reader:     resp.Body,
		total:      totalSize,
		onProgress: onProgress,
	}

	// Copy with progress
	_, err = io.Copy(out, reader)
	if err != nil {
		return fmt.Errorf("download write: %w", err)
	}

	// Close file before rename
	out.Close()

	// Rename temp file to final name
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("finalize download: %w", err)
	}

	return nil
}

// progressReader wraps an io.Reader to report progress.
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	onProgress ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}

// FormatBytes formats bytes as a human-readable string.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
