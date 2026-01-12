package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/cyber/whisper-transcribe/internal/config"
	"github.com/cyber/whisper-transcribe/internal/downloader"
	"github.com/cyber/whisper-transcribe/internal/pipeline"
	"github.com/cyber/whisper-transcribe/internal/tui"
)

var (
	cfgFile    string
	noTUI      bool
	url        string
	model      string
	timestamps bool
	outputDir  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "whisper-transcribe",
		Short: "Transcribe YouTube videos to Markdown using Whisper",
		Long: `A TUI application that downloads audio from YouTube videos
and transcribes them to properly formatted Markdown files
using local OpenAI Whisper (whisper.cpp) transcription.`,
		RunE: run,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.Flags().BoolVar(&noTUI, "no-tui", false, "run in CLI mode without TUI")
	rootCmd.Flags().StringVarP(&url, "url", "u", "", "YouTube URL to transcribe")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Whisper model (tiny, base, small, medium, large)")
	rootCmd.Flags().BoolVarP(&timestamps, "timestamps", "t", false, "include timestamps in output")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if model != "" {
		cfg.DefaultModel = model
	}
	if outputDir != "" {
		cfg.OutputDir = outputDir
	}
	if timestamps {
		cfg.Timestamps = true
	}

	if noTUI || url != "" {
		return runCLI(cfg, url)
	}

	return runTUI(cfg)
}

func runTUI(cfg *config.Config) error {
	m := tui.NewModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	_, err := p.Run()
	return err
}

func runCLI(cfg *config.Config, videoURL string) error {
	if videoURL == "" {
		return fmt.Errorf("URL is required in CLI mode (use --url)")
	}

	if err := downloader.ValidateURL(videoURL); err != nil {
		return err
	}

	transcriptionCfg := &config.TranscriptionConfig{
		URL:        videoURL,
		Model:      cfg.DefaultModel,
		Timestamps: cfg.Timestamps,
		OutputDir:  cfg.OutputDir,
	}

	events := make(chan pipeline.Event, 100)

	go func() {
		p := pipeline.New(transcriptionCfg, events)
		p.Run()
		close(events)
	}()

	for event := range events {
		switch e := event.(type) {
		case pipeline.MetadataEvent:
			fmt.Printf("Video: %s\n", e.Title)
			fmt.Printf("Channel: %s\n", e.Channel)
			fmt.Printf("Duration: %s\n\n", e.Duration)
		case pipeline.ProgressEvent:
			fmt.Printf("[%s] %s (%.0f%%)\n", e.Step, e.Message, e.Progress*100)
		case pipeline.CompletedEvent:
			fmt.Printf("\nTranscription complete!\n")
			fmt.Printf("Output: %s\n", e.OutputPath)
			fmt.Printf("Words: %d\n", e.Stats.WordCount)
		case pipeline.ErrorEvent:
			return fmt.Errorf("%s: %w", e.Step, e.Err)
		}
	}

	return nil
}
