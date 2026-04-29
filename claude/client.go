package claude

import (
	"context"
	"strings"
	"zun-talk/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const systemPrompt = "あなたはずんだもんです。語尾に「〜のだ」「〜なのだ」をつけて、ずんだもん口調で話してください。明るく元気にふるまってください。"

// 文末と判定するルーン
var sentenceEnds = map[rune]bool{
	'。': true, '！': true, '？': true, '、': true, '!': true, '?': true, '\n': true,
}

type Client struct {
	api     *anthropic.Client
	history []anthropic.MessageParam
}

func NewClient(apiKey string) *Client {
	api := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{api: api}
}

// ChatStream はレスポンスをストリーミングし、文単位で cb を呼び出す。
// cb は合成・再生のトリガーに使う。
func (c *Client) ChatStream(userInput string, cb func(sentence string) error) error {
	c.history = append(c.history, anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

	stream := c.api.Messages.NewStreaming(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.Model(config.ClaudeModel)),
		MaxTokens: anthropic.F(int64(1024)),
		System: anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(systemPrompt),
		}),
		Messages: anthropic.F(c.history),
	})

	var buf strings.Builder
	var fullReply strings.Builder

	flush := func(force bool) error {
		text := buf.String()
		runes := []rune(text)
		start := 0

		for i, r := range runes {
			if sentenceEnds[r] {
				sentence := strings.TrimSpace(string(runes[start : i+1]))
				if sentence != "" {
					if err := cb(sentence); err != nil {
						return err
					}
				}
				start = i + 1
			}
		}

		buf.Reset()
		if start < len(runes) {
			if force {
				// ストリーム終了時は残りも送出
				sentence := strings.TrimSpace(string(runes[start:]))
				if sentence != "" {
					return cb(sentence)
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
	c.history = append(c.history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(reply)))
	return nil
}
