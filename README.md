# Whisper Transcribe

A terminal user interface (TUI) application for transcribing YouTube videos
and local audio files to Markdown using local Whisper speech recognition.

## Features

- Beautiful TUI built with [Charm](https://charm.sh/) libraries
- Download and transcribe YouTube videos via yt-dlp
- Transcribe local audio files (WAV, MP3, M4A, OGG, FLAC, WebM, MP4)
- Local transcription using whisper.cpp (no cloud APIs)
- Automatic Whisper model downloading with progress display
- Multiple model sizes: tiny, base, small, medium, large
- Optional timestamp inclusion in output
- Lint-compliant Markdown output with YAML frontmatter
- CLI mode for scripting and automation

## Requirements

This project uses Nix for dependency management. The following tools are
provided automatically via the Nix flake:

- Go 1.22+
- yt-dlp
- ffmpeg
- whisper.cpp
- markdownlint-cli

## Installation

### Using Nix (Recommended)

```bash
git clone https://github.com/cyber/whisper-transcribe.git
cd whisper-transcribe
nix develop
make build
```

### Manual Installation

Ensure you have the required dependencies installed, then:

```bash
go build -o whisper-transcribe ./cmd/whisper-transcribe
```

## Usage

### TUI Mode

Launch the interactive TUI by running without arguments:

```bash
./whisper-transcribe
```

The TUI provides:

- Source type selection (YouTube URL or local file)
- Model selection with visual feedback
- Timestamp toggle
- Real-time transcription progress
- Markdown preview on completion

#### TUI Navigation

| Key | Action |
| --- | ------ |
| `Tab` / `Down` | Next field |
| `Shift+Tab` / `Up` | Previous field |
| `Left` / `Right` | Select option |
| `Space` | Toggle checkbox |
| `Enter` | Submit / Confirm |
| `q` | Quit (when not processing) |
| `Ctrl+C` | Force quit |

### CLI Mode

For scripting or headless operation, use CLI flags:

```bash
# Transcribe a YouTube video
./whisper-transcribe --url "https://www.youtube.com/watch?v=VIDEO_ID"

# Transcribe a local file
./whisper-transcribe --file /path/to/audio.wav

# With options
./whisper-transcribe \
  --url "https://www.youtube.com/watch?v=VIDEO_ID" \
  --model medium \
  --timestamps \
  --output ~/my-transcripts
```

#### CLI Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--url` | `-u` | YouTube URL to transcribe |
| `--file` | `-f` | Local audio file to transcribe |
| `--model` | `-m` | Whisper model (tiny/base/small/medium/large) |
| `--timestamps` | `-t` | Include timestamps in output |
| `--output` | `-o` | Output directory for transcripts |
| `--config` | | Path to config file |
| `--no-tui` | | Force CLI mode |

## Configuration

Configuration can be provided via file or environment variables.

### Config File

Create `~/.config/whisper-transcribe/config.yaml`:

```yaml
default_model: base
output_dir: ~/transcripts
timestamps: false
```

### Environment Variables

```bash
export WHISPER_DEFAULT_MODEL=medium
export WHISPER_OUTPUT_DIR=~/transcripts
export WHISPER_TIMESTAMPS=true
```

## Whisper Models

Models are downloaded automatically on first use. You will be prompted to
confirm the download with size information displayed.

| Model | Size | Description |
| ----- | ---- | ----------- |
| tiny | ~75 MB | Fastest, lowest accuracy |
| base | ~142 MB | Good balance for short content |
| small | ~466 MB | Better accuracy |
| medium | ~1.5 GB | High accuracy |
| large | ~2.9 GB | Highest accuracy, slowest |

Models are stored in `~/.local/share/whisper-models/`.

## Output Format

Transcripts are saved as Markdown files with YAML frontmatter:

```markdown
---
title: "Video Title"
source: "https://www.youtube.com/watch?v=VIDEO_ID"
channel: "Channel Name"
uploaded: "2024-01-15"
transcribed: "2024-01-20"
duration: "10:30"
model: "whisper-base"
---

# Video Title

> Transcribed from [Channel Name](https://youtube.com/...) on 2024-01-20

## Transcription

The transcribed content appears here...
```

## Development

```bash
# Enter development shell
nix develop

# Build
make build

# Run
make run

# Run tests
make test

# Clean build artifacts
make clean
```

## Project Structure

```text
.
├── cmd/
│   └── whisper-transcribe/
│       └── main.go              # CLI entry point
├── internal/
│   ├── config/                  # Configuration handling
│   ├── downloader/              # yt-dlp wrapper
│   ├── formatter/               # Markdown generation
│   ├── models/                  # Whisper model management
│   ├── pipeline/                # Orchestration
│   ├── transcriber/             # whisper.cpp wrapper
│   └── tui/                     # Bubble Tea TUI
│       ├── screens/             # UI screens
│       └── styles/              # Lip Gloss themes
├── flake.nix                    # Nix development environment
├── go.mod
├── Makefile
└── README.md
```

## License

MIT
