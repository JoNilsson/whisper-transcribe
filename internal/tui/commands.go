package tui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/models"
	"github.com/cyber/whisper-transcribe/internal/pipeline"
	"github.com/cyber/whisper-transcribe/internal/transcriber"
)

// RunPipeline creates a command that runs the transcription pipeline.
func RunPipeline(cfg *config.TranscriptionConfig, program *tea.Program) tea.Cmd {
	return func() tea.Msg {
		events := make(chan pipeline.Event, 100)

		go func() {
			p := pipeline.New(cfg, events)
			p.Run()
			close(events)
		}()

		go func() {
			for event := range events {
				switch e := event.(type) {
				case pipeline.MetadataEvent:
					program.Send(MetadataFetchedMsg{
						Title:    e.Title,
						Channel:  e.Channel,
						Duration: e.Duration,
					})
				case pipeline.ProgressEvent:
					program.Send(PipelineProgressMsg{
						Step:     e.Step,
						Progress: e.Progress,
						Message:  e.Message,
					})
				case pipeline.TranscriptEvent:
					program.Send(TranscriptChunkMsg{
						Text:      e.Text,
						Timestamp: e.Timestamp,
					})
				case pipeline.CompletedEvent:
					program.Send(PipelineCompletedMsg{
						OutputPath: e.OutputPath,
						Stats:      e.Stats,
					})
				case pipeline.ErrorEvent:
					program.Send(PipelineErrorMsg{
						Step: e.Step,
						Err:  e.Err,
					})
				}
			}
		}()

		return PipelineStartedMsg{}
	}
}

// OpenInEditor opens a file in the user's preferred editor.
func OpenInEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditorClosedMsg{Err: err}
	})
}

// CheckModel verifies if a model exists locally.
func CheckModel(model string) tea.Cmd {
	return func() tea.Msg {
		if err := transcriber.CheckModel(model); err != nil {
			if _, ok := err.(transcriber.ErrModelNotFound); ok {
				info, _ := models.GetModelInfo(model)
				size := "unknown"
				if info != nil {
					size = info.Size
				}
				return ModelMissingMsg{Model: model, Size: size}
			}
			return PipelineErrorMsg{Step: "model_check", Err: err}
		}
		return nil
	}
}

// DownloadModel downloads a whisper model.
func DownloadModel(model string, program *tea.Program) tea.Cmd {
	return func() tea.Msg {
		err := models.Download(model, func(downloaded, total int64) {
			progress := 0.0
			if total > 0 {
				progress = float64(downloaded) / float64(total)
			}
			program.Send(ModelDownloadProgressMsg{
				Downloaded: downloaded,
				Total:      total,
				Progress:   progress,
			})
		})

		if err != nil {
			return ModelDownloadErrorMsg{Model: model, Err: err}
		}

		return ModelDownloadCompleteMsg{Model: model}
	}
}
