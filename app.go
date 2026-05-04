//go:build !cli

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"zun-talk/config"
	"zun-talk/engine"
	"zun-talk/player"
	"zun-talk/recorder"
)

// CharacterInfo is sent to the frontend.
type CharacterInfo struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	SpeakerID int    `json:"speakerId"`
}

// App is the Wails application struct. Its exported methods are bound to the frontend.
type App struct {
	ctx       context.Context
	eng       *engine.Engine
	cfg       *config.Config
	pttRec    *recorder.AsyncRecording
	vadCancel context.CancelFunc
	mu        sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.cfg = config.Load()

	if err := config.LoadCharacters(a.cfg.CharSettingsPath, a.cfg.ConvGuidePath); err != nil {
		log.Printf("キャラクター設定エラー: %v", err)
		return
	}

	if a.cfg.AnthropicAPIKey == "" {
		log.Println("警告: ANTHROPIC_API_KEY が設定されていません")
	}

	if err := player.Init(); err != nil {
		log.Printf("player.Init: %v", err)
		return
	}

	cb := engine.Callbacks{
		OnSentence: func(text, emotion, speaker string) {
			runtime.EventsEmit(a.ctx, "message:sentence", map[string]string{
				"text":    text,
				"emotion": emotion,
				"speaker": speaker,
			})
		},
		OnStatus: func(status string) {
			runtime.EventsEmit(a.ctx, "status:changed", map[string]string{
				"status": status,
			})
		},
		OnTranscribed: func(text string) {
			runtime.EventsEmit(a.ctx, "recording:transcribed", map[string]string{
				"text": text,
			})
		},
		OnDone: func(fullText string) {
			runtime.EventsEmit(a.ctx, "message:done", map[string]string{
				"fullText": fullText,
			})
		},
	}

	eng, err := engine.New(a.cfg, config.DefaultCharKey, cb)
	if err != nil {
		log.Printf("engine.New: %v", err)
		return
	}
	a.eng = eng
}

func (a *App) shutdown(_ context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vadCancel != nil {
		a.vadCancel()
	}
}

// ── キャラクター ──────────────────────────────────────────────────────

// GetCharacters returns all available characters.
func (a *App) GetCharacters() []CharacterInfo {
	result := make([]CharacterInfo, 0, len(config.Characters))
	for key, char := range config.Characters {
		result = append(result, CharacterInfo{
			Key:       key,
			Name:      char.Name,
			SpeakerID: char.SpeakerID,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result
}

// GetActiveCharacter returns the name of the currently active character.
func (a *App) GetActiveCharacter() string {
	if a.eng == nil {
		return config.DefaultCharKey
	}
	return a.eng.ActiveCharacter().Name
}

// SetCharacter switches the active character by key.
func (a *App) SetCharacter(key string) error {
	if a.eng == nil {
		return fmt.Errorf("エンジンが初期化されていません")
	}
	return a.eng.SetCharacter(key)
}

// ── テキスト送信 ──────────────────────────────────────────────────────

// SendMessage sends a text message through the full pipeline (Claude → VoiceVox → Player).
func (a *App) SendMessage(text string) error {
	if a.eng == nil {
		return fmt.Errorf("エンジンが初期化されていません")
	}
	if text == "" {
		return nil
	}
	_, err := a.eng.ProcessText(text)
	if err != nil {
		runtime.EventsEmit(a.ctx, "error:occurred", map[string]string{
			"message": err.Error(),
		})
	}
	return err
}

// ── PTT（プッシュ・トゥ・トーク） ─────────────────────────────────────

// StartPTT begins a push-to-talk recording session.
func (a *App) StartPTT() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pttRec != nil {
		return nil
	}
	if a.vadCancel != nil {
		return fmt.Errorf("VAD モード中は PTT を使用できません")
	}

	rec, err := recorder.StartAsync()
	if err != nil {
		return fmt.Errorf("録音開始エラー: %w", err)
	}
	a.pttRec = rec
	runtime.EventsEmit(a.ctx, "status:changed", map[string]string{"status": "recording"})
	return nil
}

// StopPTT ends the PTT session, transcribes, and sends the message.
// Returns immediately; processing happens in the background.
func (a *App) StopPTT() error {
	a.mu.Lock()
	rec := a.pttRec
	a.pttRec = nil
	a.mu.Unlock()

	if rec == nil {
		return nil
	}

	wav, err := rec.Stop()
	if err != nil {
		runtime.EventsEmit(a.ctx, "status:changed", map[string]string{"status": "idle"})
		return fmt.Errorf("録音停止エラー: %w", err)
	}

	if a.eng == nil {
		return fmt.Errorf("エンジンが初期化されていません")
	}

	go func() {
		if err := a.eng.ProcessVoice(wav); err != nil {
			runtime.EventsEmit(a.ctx, "error:occurred", map[string]string{
				"message": err.Error(),
			})
		}
	}()
	return nil
}

// ── VAD（音声自動検知） ───────────────────────────────────────────────

// SetVAD starts or stops the continuous voice activity detection loop.
func (a *App) SetVAD(enabled bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !enabled {
		if a.vadCancel != nil {
			a.vadCancel()
			a.vadCancel = nil
		}
		runtime.EventsEmit(a.ctx, "status:changed", map[string]string{"status": "idle"})
		return nil
	}

	if a.vadCancel != nil {
		return nil
	}
	if a.pttRec != nil {
		return fmt.Errorf("PTT 録音中は VAD を開始できません")
	}
	if a.eng == nil {
		return fmt.Errorf("エンジンが初期化されていません")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.vadCancel = cancel
	go a.runVADLoop(ctx)
	return nil
}

func (a *App) runVADLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		runtime.EventsEmit(a.ctx, "status:changed", map[string]string{"status": "recording"})

		wav, err := recorder.RecordVADWithContext(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			runtime.EventsEmit(a.ctx, "error:occurred", map[string]string{
				"message": err.Error(),
			})
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := a.eng.ProcessVoice(wav); err != nil {
			runtime.EventsEmit(a.ctx, "error:occurred", map[string]string{
				"message": err.Error(),
			})
		}

		// Brief pause before next listen cycle
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}
