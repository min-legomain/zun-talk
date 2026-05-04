package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed character-settings.md
var defaultCharSettings []byte

//go:embed conversation-guide.md
var defaultConvGuide []byte

const (
	ClaudeModel             = "claude-sonnet-4-6"
	DefaultVoicevoxURL      = "http://localhost:50021"
	DefaultCharKey          = "zundamon"
	DefaultCharSettingsPath = "./config/character-settings.md"
	DefaultConvGuidePath    = "./config/conversation-guide.md"
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

// LoadCharacters はファイルからキャラクター設定を読み込む。
// ファイルが見つからない場合はバイナリに埋め込まれたデフォルト設定を使う。
func LoadCharacters(charPath, guidePath string) error {
	charData, err := os.ReadFile(charPath)
	if err != nil {
		charData = defaultCharSettings
	}

	guideData, err := os.ReadFile(guidePath)
	if err != nil {
		guideData = defaultConvGuide
	}

	return parseCharacters(charData, guideData)
}

func parseCharacters(charData, guideData []byte) error {
	guide := "\n\n" + strings.TrimSpace(string(guideData))

	jpNameToKey := make(map[string]string, len(characterDefs))
	keyToSpeakerID := make(map[string]int, len(characterDefs))
	for _, d := range characterDefs {
		jpNameToKey[d.jpName] = d.key
		keyToSpeakerID[d.key] = d.speakerID
	}

	content := string(charData)
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
		prompt := "あなたは" + name + "です。以下の設定に従って話してください。\n\n" + body + guide
		chars[key] = Character{
			Name:      name,
			SpeakerID: keyToSpeakerID[key],
			Prompt:    prompt,
		}
	}

	if len(chars) == 0 {
		return fmt.Errorf("キャラクターが見つかりません")
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
	ConvGuidePath    string
}

func Load() *Config {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = readAPIKeyFromFile()
	}

	return &Config{
		AnthropicAPIKey:  apiKey,
		WhisperBinary:    envOr("WHISPER_BINARY", DefaultWhisperBinary),
		WhisperModel:     envOr("WHISPER_MODEL", DefaultWhisperModel),
		VoicevoxURL:      envOr("VOICEVOX_URL", DefaultVoicevoxURL),
		CharSettingsPath: envOr("CHAR_SETTINGS", DefaultCharSettingsPath),
		ConvGuidePath:    envOr("CONV_GUIDE", DefaultConvGuidePath),
	}
}

// readAPIKeyFromFile は GUI アプリ向けに設定ファイルから API キーを読む。
// ~/Library/Application Support/zun-talk/.env または ~/.config/zun-talk/.env を参照する。
func readAPIKeyFromFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, "Library", "Application Support", "zun-talk", ".env"),
		filepath.Join(home, ".config", "zun-talk", ".env"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(line, "ANTHROPIC_API_KEY="); ok {
				return strings.Trim(after, `"'`)
			}
		}
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
