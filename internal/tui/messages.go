package tui

import "github.com/cyber/whisper-transcribe/internal/pipeline"

// Screen represents the current TUI screen.
type Screen int

const (
	InputScreen Screen = iota
	ProgressScreen
	PreviewScreen
)

// ScreenMsg triggers a screen transition.
type ScreenMsg Screen

// PipelineStartedMsg signals the pipeline has started.
type PipelineStartedMsg struct{}

// MetadataFetchedMsg contains video metadata.
type MetadataFetchedMsg struct {
	Title    string
	Channel  string
	Duration string
}

// PipelineProgressMsg reports step progress.
type PipelineProgressMsg struct {
	Step     string
	Progress float64
	Message  string
}

// TranscriptChunkMsg streams transcription text.
type TranscriptChunkMsg struct {
	Text      string
	Timestamp string
}

// PipelineCompletedMsg signals successful completion.
type PipelineCompletedMsg struct {
	OutputPath string
	Stats      pipeline.Stats
}

// PipelineErrorMsg signals a pipeline error.
type PipelineErrorMsg struct {
	Step string
	Err  error
}

// EditorClosedMsg signals the external editor has closed.
type EditorClosedMsg struct {
	Err error
}

// StartNewMsg triggers return to input screen.
type StartNewMsg struct{}
