package engine

import (
	"fmt"
	"strings"

	"zun-talk/claude"
	"zun-talk/config"
	"zun-talk/player"
	"zun-talk/stt"
	"zun-talk/voicevox"
)

type sentenceEvent struct {
	text    string
	emotion string
}

// Callbacks are optional hooks called during pipeline execution.
type Callbacks struct {
	OnSentence    func(text, emotion, speaker string)
	OnStatus      func(status string)
	OnTranscribed func(text string)
	OnDone        func(fullText string)
}

// Engine orchestrates the Claude → VoiceVox → Player pipeline.
type Engine struct {
	cfg            *config.Config
	claudeClient   *claude.Client
	voicevoxClient *voicevox.Client
	whisperClient  *stt.Client
	activeChar     config.Character
	cb             Callbacks
}

func New(cfg *config.Config, charKey string, cb Callbacks) (*Engine, error) {
	char, ok := config.Characters[charKey]
	if !ok {
		return nil, fmt.Errorf("不明なキャラクター: %s", charKey)
	}
	return &Engine{
		cfg:            cfg,
		claudeClient:   claude.NewClient(cfg.AnthropicAPIKey, char.Prompt),
		voicevoxClient: voicevox.NewClient(cfg.VoicevoxURL, char.SpeakerID),
		whisperClient:  stt.NewClient(cfg.WhisperBinary, cfg.WhisperModel),
		activeChar:     char,
		cb:             cb,
	}, nil
}

func (e *Engine) ActiveCharacter() config.Character {
	return e.activeChar
}

func (e *Engine) Config() *config.Config {
	return e.cfg
}

func (e *Engine) SetCharacter(key string) error {
	char, ok := config.Characters[key]
	if !ok {
		return fmt.Errorf("不明なキャラクター: %s", key)
	}
	e.activeChar = char
	e.claudeClient.SetSystemPrompt(char.Prompt)
	e.voicevoxClient.SetSpeakerID(char.SpeakerID)
	return nil
}

// NewClaudeClient creates an independent Claude client for the given character.
// Used for duet mode.
func (e *Engine) NewClaudeClient(charKey string) (*claude.Client, error) {
	char, ok := config.Characters[charKey]
	if !ok {
		return nil, fmt.Errorf("不明なキャラクター: %s", charKey)
	}
	return claude.NewClient(e.cfg.AnthropicAPIKey, char.Prompt), nil
}

// NewVoicevoxClient creates an independent VoiceVox client for the given character.
// Used for duet mode.
func (e *Engine) NewVoicevoxClient(charKey string) (*voicevox.Client, error) {
	char, ok := config.Characters[charKey]
	if !ok {
		return nil, fmt.Errorf("不明なキャラクター: %s", charKey)
	}
	return voicevox.NewClient(e.cfg.VoicevoxURL, char.SpeakerID), nil
}

// ProcessText runs the full pipeline with the engine's active character.
func (e *Engine) ProcessText(text string) (string, error) {
	return e.processText(e.claudeClient, e.voicevoxClient, e.activeChar.Name, text)
}

// ProcessTextWith runs the pipeline with the provided clients and speaker name.
// Used for duet mode.
func (e *Engine) ProcessTextWith(cc *claude.Client, vc *voicevox.Client, speaker, text string) (string, error) {
	return e.processText(cc, vc, speaker, text)
}

func (e *Engine) processText(cc *claude.Client, vc *voicevox.Client, speaker, text string) (string, error) {
	if e.cb.OnStatus != nil {
		e.cb.OnStatus("thinking")
	}

	sentenceCh := make(chan sentenceEvent, 8)
	wavCh := make(chan []byte, 4)
	streamErrCh := make(chan error, 1)
	synthErrCh := make(chan error, 1)
	playErrCh := make(chan error, 1)

	var fullResponse strings.Builder

	// Stage 1: stream Claude response sentence by sentence
	go func() {
		err := cc.ChatStream(text, func(sentence, emotion string) error {
			fullResponse.WriteString(sentence)
			if e.cb.OnSentence != nil {
				e.cb.OnSentence(sentence, emotion, speaker)
			}
			sentenceCh <- sentenceEvent{text: sentence, emotion: emotion}
			return nil
		})
		close(sentenceCh)
		streamErrCh <- err
	}()

	// Stage 2: synthesize each sentence to WAV
	go func() {
		var firstErr error
		for se := range sentenceCh {
			if firstErr != nil {
				continue
			}
			wav, err := vc.Synthesize(se.text, se.emotion)
			if err != nil {
				firstErr = err
				continue
			}
			wavCh <- wav
		}
		close(wavCh)
		synthErrCh <- firstErr
	}()

	// Stage 3: play WAVs sequentially
	go func() {
		var firstErr error
		for wav := range wavCh {
			if e.cb.OnStatus != nil {
				e.cb.OnStatus("speaking")
			}
			if firstErr == nil {
				if err := player.PlayWAV(wav); err != nil {
					firstErr = err
				}
			}
		}
		playErrCh <- firstErr
	}()

	streamErr := <-streamErrCh
	synthErr := <-synthErrCh
	playErr := <-playErrCh

	fullText := fullResponse.String()
	if e.cb.OnDone != nil {
		e.cb.OnDone(fullText)
	}
	if e.cb.OnStatus != nil {
		e.cb.OnStatus("idle")
	}

	if streamErr != nil {
		return "", fmt.Errorf("Claude API: %w", streamErr)
	}
	if synthErr != nil {
		return "", fmt.Errorf("VOICEVOX: %w", synthErr)
	}
	if playErr != nil {
		return "", fmt.Errorf("再生エラー: %w", playErr)
	}
	return fullText, nil
}

// ProcessVoice transcribes audio then sends the result through ProcessText.
func (e *Engine) ProcessVoice(wavData []byte) error {
	if e.cb.OnStatus != nil {
		e.cb.OnStatus("transcribing")
	}
	text, err := e.whisperClient.Transcribe(wavData)
	if err != nil {
		if e.cb.OnStatus != nil {
			e.cb.OnStatus("idle")
		}
		return fmt.Errorf("Whisper: %w", err)
	}
	if text == "" {
		if e.cb.OnStatus != nil {
			e.cb.OnStatus("idle")
		}
		return nil
	}
	if e.cb.OnTranscribed != nil {
		e.cb.OnTranscribed(text)
	}
	_, err = e.ProcessText(text)
	return err
}
