package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	ClaudeModel             = "claude-sonnet-4-6"
	DefaultVoicevoxURL      = "http://localhost:50021"
	DefaultCharKey          = "zundamon"
	DefaultCharSettingsPath = "./config/character-settings.md"
	DefaultWhisperBinary    = "./whisper-cpp"
	DefaultWhisperModel     = "./models/ggml-medium.bin"
)

type characterDef struct {
	jpName    string
	key       string
	speakerID int
}

var characterDefs = []characterDef{
	{jpName: "ずんだもん", key: "zundamon", speakerID: 3},
	{jpName: "四国めたん", key: "metan", speakerID: 2},
	{jpName: "春日部つむぎ", key: "tsumugi", speakerID: 8},
}

type Character struct {
	Name      string
	SpeakerID int
	Prompt    string
}

var Characters map[string]Character

func LoadCharacters(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("character settings: %w", err)
	}

	jpNameToKey := make(map[string]string, len(characterDefs))
	keyToSpeakerID := make(map[string]int, len(characterDefs))
	for _, d := range characterDefs {
		jpNameToKey[d.jpName] = d.key
		keyToSpeakerID[d.key] = d.speakerID
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

		key, ok := jpNameToKey[name]
		if !ok {
			continue
		}
		prompt := "あなたは" + name + "です。以下の設定に従って話してください。\n\n" + body
		chars[key] = Character{
			Name:      name,
			SpeakerID: keyToSpeakerID[key],
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
	return &Config{
		AnthropicAPIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		WhisperBinary:    envOr("WHISPER_BINARY", DefaultWhisperBinary),
		WhisperModel:     envOr("WHISPER_MODEL", DefaultWhisperModel),
		VoicevoxURL:      envOr("VOICEVOX_URL", DefaultVoicevoxURL),
		CharSettingsPath: envOr("CHAR_SETTINGS", DefaultCharSettingsPath),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
