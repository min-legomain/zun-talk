package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	ClaudeModel          = "claude-sonnet-4-6"
	WhisperBinaryPath    = "./whisper-cpp"
	WhisperModelPath     = "./models/ggml-medium.bin"
	DefaultVoicevoxURL   = "http://localhost:50021"
	DefaultCharKey       = "zundamon"
	DefaultCharSettingsPath = "./config/character-settings.md"
)

// nameToKey はmarkdown上のキャラ名をコード上のキーに対応させる
var nameToKey = map[string]string{
	"ずんだもん":   "zundamon",
	"四国めたん":   "metan",
	"春日部つむぎ": "tsumugi",
}

// speakerIDs はVOICEVOXのスピーカーID
var speakerIDs = map[string]int{
	"zundamon": 3,
	"metan":    2,
	"tsumugi":  8,
}

type Character struct {
	Name      string
	SpeakerID int
	Prompt    string
}

var Characters map[string]Character

// LoadCharacters はmarkdownファイルを読み込みCharactersを初期化する
func LoadCharacters(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("character settings: %w", err)
	}

	content := string(data)
	if strings.HasPrefix(content, "## ") {
		content = "\n" + content
	}
	parts := strings.Split(content, "\n## ")

	chars := make(map[string]Character)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		nl := strings.Index(part, "\n")
		if nl < 0 {
			continue
		}
		name := strings.TrimSpace(part[:nl])
		body := strings.TrimSpace(part[nl+1:])

		key, ok := nameToKey[name]
		if !ok {
			continue
		}
		prompt := "あなたは" + name + "です。以下の設定に従って話してください。\n\n" + body
		chars[key] = Character{
			Name:      name,
			SpeakerID: speakerIDs[key],
			Prompt:    prompt,
		}
	}

	if len(chars) == 0 {
		return fmt.Errorf("キャラクターが見つかりません: %s", path)
	}
	Characters = chars
	return nil
}

type Config struct {
	AnthropicAPIKey  string
	WhisperBinary    string
	WhisperModel     string
	VoicevoxURL      string
	CharSettingsPath string
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

	charSettings := os.Getenv("CHAR_SETTINGS")
	if charSettings == "" {
		charSettings = DefaultCharSettingsPath
	}

	return &Config{
		AnthropicAPIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		WhisperBinary:    whisperBinary,
		WhisperModel:     whisperModel,
		VoicevoxURL:      voicevoxURL,
		CharSettingsPath: charSettings,
	}
}
