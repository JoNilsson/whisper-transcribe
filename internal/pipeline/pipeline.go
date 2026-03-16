package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/formatter"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

// Event represents a pipeline event.
type Event interface {
	isEvent()
}

// MetadataEvent is sent when video metadata is fetched.
type MetadataEvent struct {
	Title    string
	Channel  string
	Duration string
}

func (MetadataEvent) isEvent() {}

// ProgressEvent reports step progress.
type ProgressEvent struct {
	Step     string
	Progress float64
	Message  string
}

func (ProgressEvent) isEvent() {}

// TranscriptEvent streams transcription chunks.
type TranscriptEvent struct {
	Text      string
	Timestamp string
}

func (TranscriptEvent) isEvent() {}

// CompletedEvent signals successful completion.
type CompletedEvent struct {
	OutputPath string
	Stats      Stats
}

func (CompletedEvent) isEvent() {}

// ErrorEvent signals a pipeline error.
type ErrorEvent struct {
	Step string
	Err  error
}

func (ErrorEvent) isEvent() {}

// Stats holds transcription statistics.
type Stats struct {
	Duration  string
	WordCount int
	Model     string
	AudioPath string
	RawPath   string
}

// Pipeline orchestrates the transcription workflow.
type Pipeline struct {
	config *config.TranscriptionConfig
	events chan<- Event
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new pipeline.
func New(cfg *config.TranscriptionConfig, events chan<- Event) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pipeline{
		config: cfg,
		events: events,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run executes the pipeline steps.
func (p *Pipeline) Run() {
	defer p.cancel()

	var meta *downloader.Metadata
	var audioPath string
	var err error

	if p.config.IsLocalFile() {
		// Local file: create metadata from filename
		meta = createLocalMetadata(p.config.LocalFile)
		audioPath = p.config.LocalFile

		p.events <- MetadataEvent{
			Title:    meta.Title,
			Channel:  meta.Channel,
			Duration: meta.Duration,
		}
		p.events <- ProgressEvent{Step: "metadata", Progress: 1.0, Message: "Local file ready"}
	} else {
		// Step 1: Fetch metadata
		p.events <- ProgressEvent{Step: "metadata", Progress: 0, Message: "Fetching video info..."}
		meta, err = downloader.FetchMetadata(p.ctx, p.config.URL)
		if err != nil {
			p.events <- ErrorEvent{Step: "metadata", Err: err}
			return
		}
		p.events <- MetadataEvent{
			Title:    meta.Title,
			Channel:  meta.Channel,
			Duration: meta.Duration,
		}
		p.events <- ProgressEvent{Step: "metadata", Progress: 1.0, Message: "Done"}

		// Step 2: Download audio
		p.events <- ProgressEvent{Step: "download", Progress: 0, Message: "Starting download..."}
		audioPath, err = downloader.Download(p.ctx, p.config.URL, func(progress float64) {
			p.events <- ProgressEvent{Step: "download", Progress: progress, Message: "Downloading..."}
		})
		if err != nil {
			p.events <- ErrorEvent{Step: "download", Err: err}
			return
		}
		p.events <- ProgressEvent{Step: "download", Progress: 1.0, Message: "Done"}
	}

	// Copy audio to output directory (early, so it's saved even if user bails)
	slug := formatter.Slugify(meta.Title)
	if err := os.MkdirAll(p.config.OutputDir, 0755); err != nil {
		p.events <- ErrorEvent{Step: "format", Err: fmt.Errorf("create output dir: %w", err)}
		return
	}

	var savedAudioPath string
	if !p.config.IsLocalFile() && audioPath != "" {
		ext := filepath.Ext(audioPath)
		destPath := filepath.Join(p.config.OutputDir, slug+ext)
		if err := copyFile(audioPath, destPath); err == nil {
			savedAudioPath = destPath
		}
	} else if p.config.IsLocalFile() {
		savedAudioPath = audioPath
	}

	// Open raw transcript file for incremental writes
	rawPath := filepath.Join(p.config.OutputDir, slug+".txt")
	rawFile, err := os.Create(rawPath)
	if err != nil {
		rawPath = "" // non-fatal
	}

	// Step 3: Transcribe
	p.events <- ProgressEvent{Step: "transcribe", Progress: 0, Message: "Starting transcription..."}
	segments, err := transcriber.Transcribe(p.ctx, audioPath, p.config.Model, func(chunk transcriber.Chunk) {
		p.events <- TranscriptEvent{
			Text:      chunk.Text,
			Timestamp: chunk.Timestamp,
		}
		p.events <- ProgressEvent{
			Step:     "transcribe",
			Progress: chunk.Progress,
			Message:  "Transcribing...",
		}
		// Append chunk to raw transcript file immediately
		if rawFile != nil {
			if p.config.Timestamps {
				fmt.Fprintf(rawFile, "[%s] %s\n", chunk.Timestamp, chunk.Text)
			} else {
				fmt.Fprintf(rawFile, "%s\n", chunk.Text)
			}
		}
	})
	if rawFile != nil {
		rawFile.Close()
	}
	if err != nil {
		p.events <- ErrorEvent{Step: "transcribe", Err: err}
		return
	}
	p.events <- ProgressEvent{Step: "transcribe", Progress: 1.0, Message: "Done"}

	// Step 4: Format markdown
	p.events <- ProgressEvent{Step: "format", Progress: 0, Message: "Generating markdown..."}
	outputPath, err := formatter.GenerateMarkdown(meta, segments, p.config, func(progress float64, message string) {
		p.events <- ProgressEvent{
			Step:     "format",
			Progress: progress,
			Message:  message,
		}
	})
	if err != nil {
		p.events <- ErrorEvent{Step: "format", Err: err}
		return
	}
	p.events <- ProgressEvent{Step: "format", Progress: 1.0, Message: "Done"}

	// Step 5: Validate
	p.events <- ProgressEvent{Step: "validate", Progress: 0, Message: "Checking markdown..."}
	if err := formatter.LintMarkdown(outputPath); err != nil {
		// Log warning but don't fail
		p.events <- ProgressEvent{Step: "validate", Progress: 1.0, Message: "Warnings found"}
	} else {
		p.events <- ProgressEvent{Step: "validate", Progress: 1.0, Message: "Passed"}
	}

	// Complete
	p.events <- CompletedEvent{
		OutputPath: outputPath,
		Stats: Stats{
			Duration:  meta.Duration,
			WordCount: transcriber.CountWords(segments),
			Model:     p.config.Model,
			AudioPath: savedAudioPath,
			RawPath:   rawPath,
		},
	}
}

// Cancel stops the pipeline.
func (p *Pipeline) Cancel() {
	p.cancel()
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// createLocalMetadata generates metadata from a local file path.
func createLocalMetadata(filePath string) *downloader.Metadata {
	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	title := filename[:len(filename)-len(ext)]

	// Clean up the title (replace underscores/hyphens with spaces)
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")

	return &downloader.Metadata{
		Title:      title,
		Channel:    "Local File",
		ChannelURL: "",
		Duration:   "Unknown",
		UploadDate: time.Now().Format("20060102"),
	}
}
