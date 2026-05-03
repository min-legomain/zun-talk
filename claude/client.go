package claude

import (
	"context"
	"strings"
	"zun-talk/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

var sentenceEnds = map[rune]bool{
	'。': true, '！': true, '？': true, '、': true, '!': true, '?': true, '\n': true,
}

type Client struct {
	api          *anthropic.Client
	history      []anthropic.MessageParam
	systemPrompt string
}

func NewClient(apiKey, systemPrompt string) *Client {
	api := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{api: api, systemPrompt: systemPrompt}
}

// SetSystemPrompt はキャラクターを切り替える。会話履歴はリセットされる。
func (c *Client) SetSystemPrompt(prompt string) {
	c.systemPrompt = prompt
	c.history = nil
}

// ChatStream はレスポンスをストリーミングし、文単位で cb を呼び出す。
// レスポンス冒頭の [E:感情名] タグを抽出し emotion として cb に渡す。
func (c *Client) ChatStream(userInput string, cb func(sentence, emotion string) error) error {
	c.history = append(c.history, anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

	stream := c.api.Messages.NewStreaming(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.Model(config.ClaudeModel)),
		MaxTokens: anthropic.F(int64(1024)),
		System: anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(c.systemPrompt),
		}),
		Messages: anthropic.F(c.history),
	})

	var buf strings.Builder
	var fullReply strings.Builder
	var emotion string
	emotionParsed := false

	const emotionPrefix = "[E:"

	// 返答冒頭の [E:xxx] タグを抽出して buf から除去する。
	// タグがまだ完全に届いていない場合は false を返し、呼び出し側は待機する。
	extractEmotion := func() bool {
		if emotionParsed {
			return true
		}
		text := buf.String()
		if len(text) >= len(emotionPrefix) {
			if !strings.HasPrefix(text, emotionPrefix) {
				emotionParsed = true
				return true
			}
			if end := strings.Index(text, "]"); end > 0 {
				emotion = text[len(emotionPrefix):end]
				remaining := strings.TrimLeft(text[end+1:], " \n")
				buf.Reset()
				buf.WriteString(remaining)
				emotionParsed = true
				return true
			}
			return false // "]" 未着
		}
		// text が emotionPrefix の前方一致なら待機
		if strings.HasPrefix(emotionPrefix, text) {
			return false
		}
		emotionParsed = true
		return true
	}

	flush := func(force bool) error {
		if !extractEmotion() {
			return nil
		}
		text := buf.String()
		runes := []rune(text)
		start := 0

		for i, r := range runes {
			if sentenceEnds[r] {
				sentence := strings.TrimSpace(string(runes[start : i+1]))
				if sentence != "" {
					if err := cb(sentence, emotion); err != nil {
						return err
					}
				}
				start = i + 1
			}
		}

		buf.Reset()
		if start < len(runes) {
			if force {
				sentence := strings.TrimSpace(string(runes[start:]))
				if sentence != "" {
					return cb(sentence, emotion)
				}
			} else {
				buf.WriteString(string(runes[start:]))
			}
		}
		return nil
	}

	for stream.Next() {
		event := stream.Current()
		if e, ok := event.AsUnion().(anthropic.ContentBlockDeltaEvent); ok {
			if delta, ok := e.Delta.AsUnion().(anthropic.TextDelta); ok {
				buf.WriteString(delta.Text)
				fullReply.WriteString(delta.Text)
				if err := flush(false); err != nil {
					return err
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return err
	}

	if err := flush(true); err != nil {
		return err
	}

	reply := fullReply.String()
	// 履歴に保存する前に感情タグを除去する
	if strings.HasPrefix(reply, emotionPrefix) {
		if end := strings.Index(reply, "]"); end > 0 {
			reply = strings.TrimLeft(reply[end+1:], " \n")
		}
	}
	c.history = append(c.history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(reply)))
	return nil
}
