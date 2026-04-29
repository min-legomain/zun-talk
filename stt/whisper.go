package stt

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Client struct {
	binaryPath string
	modelPath  string
}

func NewClient(binaryPath, modelPath string) *Client {
	return &Client{binaryPath: binaryPath, modelPath: modelPath}
}

func (c *Client) Transcribe(wavData []byte) (string, error) {
	f, err := os.CreateTemp("", "whisper-*.wav")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(wavData); err != nil {
		f.Close()
		return "", fmt.Errorf("write wav: %w", err)
	}
	f.Close()

	cmd := exec.Command(c.binaryPath,
		"-m", c.modelPath,
		"-l", "ja",
		"--no-timestamps",
		"-f", f.Name(),
	)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("whisper: %w", err)
	}

	text := strings.TrimSpace(string(out))
	return text, nil
}
