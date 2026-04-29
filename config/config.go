package config

import "os"

const (
	VoicevoxSpeakerID  = 3
	ClaudeModel        = "claude-sonnet-4-6"
	WhisperBinaryPath  = "./whisper-cpp"
	WhisperModelPath   = "./models/ggml-medium.bin"
	DefaultVoicevoxURL = "http://localhost:50021"
)

type Config struct {
	AnthropicAPIKey string
	WhisperBinary   string
	WhisperModel    string
	VoicevoxURL     string
}

func Load() *Config {
	whisperBinary := os.Getenv("WHISPER_BINARY")
	if whisperBinary == "" {
		whisperBinary = WhisperBinaryPath
	}

	whisperModel := os.Getenv("WHISPER_MODEL")
	if whisperModel == "" {
		whisperModel = WhisperModelPath
	}

	voicevoxURL := os.Getenv("VOICEVOX_URL")
	if voicevoxURL == "" {
		voicevoxURL = DefaultVoicevoxURL
	}

	return &Config{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		WhisperBinary:   whisperBinary,
		WhisperModel:    whisperModel,
		VoicevoxURL:     voicevoxURL,
	}
}
