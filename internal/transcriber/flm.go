package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	flmHost     = "127.0.0.1"
	flmPort     = "52627" // dedicated ASR port to avoid conflicts with LLM/embedding services
	flmModelTag = "whisper-v3:turbo" // for flm pull / flm list
	flmAPIModel = "whisper-v3"       // for /v1/audio/transcriptions requests
)

type flmResponse struct {
	Text     string       `json:"text"`
	Segments []flmSegment `json:"segments"`
	Duration float64      `json:"duration"`
}

type flmSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// IsNPUModel returns true if the model should use the FLM NPU backend.
func IsNPUModel(model string) bool {
	return model == "npu"
}

// FLMAvailable returns true if the flm binary is found in PATH.
func FLMAvailable() bool {
	_, err := exec.LookPath("flm")
	return err == nil
}

// CheckFLMModel checks whether the FLM whisper model is installed.
func CheckFLMModel() error {
	flmBin, err := exec.LookPath("flm")
	if err != nil {
		return fmt.Errorf("flm not found in PATH")
	}

	output, err := exec.Command(flmBin, "list", "--json").Output()
	if err != nil {
		return fmt.Errorf("flm list: %w", err)
	}

	var result struct {
		Models []struct {
			Name      string `json:"name"`
			Installed bool   `json:"installed"`
		} `json:"models"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("parse flm list: %w", err)
	}

	for _, m := range result.Models {
		if m.Name == flmModelTag {
			if m.Installed {
				return nil
			}
			return ErrModelNotFound{Model: "npu"}
		}
	}

	return fmt.Errorf("whisper model not available in FLM (requires flm >= 0.9.14)")
}

// TranscribeFLM performs transcription using FLM's NPU-accelerated Whisper via its OpenAI-compatible API.
func TranscribeFLM(ctx context.Context, audioPath string, onChunk ChunkFunc) ([]Segment, error) {
	baseURL := fmt.Sprintf("http://%s:%s", flmHost, flmASRPort())

	if err := ensureFLMServer(ctx, baseURL); err != nil {
		return nil, err
	}

	// Signal that transcription has started
	if onChunk != nil {
		onChunk(Chunk{Progress: 0.05})
	}

	resp, err := callFLMTranscribe(ctx, baseURL, audioPath)
	if err != nil {
		return nil, err
	}

	// If API returned text but no segments, create a single segment
	if len(resp.Segments) == 0 && resp.Text != "" {
		resp.Segments = []flmSegment{{
			Start: 0,
			End:   resp.Duration,
			Text:  resp.Text,
		}}
	}

	var segments []Segment
	total := len(resp.Segments)
	for i, seg := range resp.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		s := Segment{
			Start:     fmtTimestamp(seg.Start),
			End:       fmtTimestamp(seg.End),
			Text:      text,
			Timestamp: fmtTimestampShort(seg.Start),
		}
		segments = append(segments, s)
		if onChunk != nil {
			progress := float64(i+1) / float64(total)
			if progress > 0.99 {
				progress = 0.99
			}
			onChunk(Chunk{
				Text:      s.Text,
				Timestamp: s.Timestamp,
				Progress:  progress,
			})
		}
	}

	return segments, nil
}

func flmASRPort() string {
	if p := os.Getenv("FLM_ASR_PORT"); p != "" {
		return p
	}
	return flmPort
}

func ensureFLMServer(ctx context.Context, baseURL string) error {
	port := flmASRPort()
	if isPortOpen(flmHost, port) {
		return nil
	}

	flmBin, err := exec.LookPath("flm")
	if err != nil {
		return fmt.Errorf("flm not found in PATH")
	}

	cmd := exec.Command(flmBin, "serve", "--asr", "1", "--port", port)
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Detach from parent process group so it survives app exit
	cmd.SysProcAttr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start flm server: %w", err)
	}
	go cmd.Wait() // reap process without blocking

	// Wait for server readiness (up to 30s)
	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		if isPortOpen(flmHost, port) {
			time.Sleep(1 * time.Second) // let it finish init
			return nil
		}
	}

	return fmt.Errorf("flm server did not start within 30 seconds")
}

func isPortOpen(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func callFLMTranscribe(ctx context.Context, baseURL, audioPath string) (*flmResponse, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	writer.WriteField("model", flmAPIModel)
	writer.WriteField("response_format", "verbose_json")
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer flm")

	client := &http.Client{Timeout: 30 * time.Minute}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("FLM API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var result flmResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

func fmtTimestamp(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func fmtTimestampShort(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	if h > 0 {
		return fmt.Sprintf("[%02d:%02d:%02d]", h, m, s)
	}
	return fmt.Sprintf("[%02d:%02d]", m, s)
}
