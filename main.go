package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"zun-talk/claude"
	"zun-talk/config"
	"zun-talk/player"
	"zun-talk/recorder"
	"zun-talk/stt"
	"zun-talk/voicevox"
)

type inputMode int

const (
	modeText inputMode = iota
	modePTT
	modeVAD
)

func (m inputMode) String() string {
	switch m {
	case modeText:
		return "text"
	case modePTT:
		return "ptt"
	case modeVAD:
		return "vad"
	}
	return "unknown"
}

type sentenceEvent struct {
	text    string
	emotion string
}

func main() {
	charKey := flag.String("char", config.DefaultCharKey, "キャラクター選択: zundamon / tsumugi / metan")
	flag.Parse()

	cfg := config.Load()

	if err := config.LoadCharacters(cfg.CharSettingsPath); err != nil {
		log.Fatalf("キャラクター設定の読み込みに失敗: %v", err)
	}

	char, ok := config.Characters[*charKey]
	if !ok {
		log.Fatalf("不明なキャラクター: %s\n使えるキャラ: zundamon, tsumugi, metan", *charKey)
	}

	if cfg.AnthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY が設定されていません")
	}

	if err := player.Init(); err != nil {
		log.Fatalf("player.Init: %v", err)
	}

	claudeClient := claude.NewClient(cfg.AnthropicAPIKey, char.Prompt)
	voicevoxClient := voicevox.NewClient(cfg.VoicevoxURL, char.SpeakerID)
	whisperClient := stt.NewClient(cfg.WhisperBinary, cfg.WhisperModel)

	switchChar := func(key string) {
		c, ok := config.Characters[key]
		if !ok {
			fmt.Printf("不明なキャラクター: %s\n使えるキャラ: zundamon, tsumugi, metan\n", key)
			return
		}
		char = c
		claudeClient.SetSystemPrompt(c.Prompt)
		voicevoxClient.SetSpeakerID(c.SpeakerID)
		fmt.Printf("キャラクター変更: %s\n", c.Name)
	}

	mode := modeText
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("zun-talk へようこそ！（キャラ: %s）\n", char.Name)
	fmt.Printf("現在のモード: %s\n", mode)
	fmt.Println("モード切替: :text / :ptt / :vad | キャラ切替: :char <名前> | 終了: :quit")
	fmt.Println()

	for {
		if mode == modeVAD {
			fmt.Println("話しかけてください | :text / :ptt / :quit を入力して切り替え")
			wavData, err := recorder.RecordVAD()
			if err != nil {
				fmt.Fprintf(os.Stderr, "録音エラー: %v\n", err)
				continue
			}
			if err := processVoice(claudeClient, voicevoxClient, whisperClient, char.Name, wavData); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
			continue
		}

		if mode == modePTT {
			fmt.Println("Enterを押して録音 | :char <名前> でキャラ切替 | :text / :vad / :quit")
		}
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, ":char") {
			handleCharCommand(line, switchChar, &char)
			continue
		}
		if handleCommand(&mode, line) {
			continue
		}
		if line == "" {
			continue
		}

		if mode == modePTT {
			wavData, err := recorder.RecordPushToTalk()
			if err != nil {
				fmt.Fprintf(os.Stderr, "録音エラー: %v\n", err)
				continue
			}
			if err := processVoice(claudeClient, voicevoxClient, whisperClient, char.Name, wavData); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
		} else {
			if _, err := processText(claudeClient, voicevoxClient, char.Name, line); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
		}
	}
}

func handleCharCommand(line string, switchChar func(string), char *config.Character) {
	key := strings.TrimSpace(strings.TrimPrefix(line, ":char"))
	if key == "" {
		fmt.Printf("現在のキャラ: %s\n使えるキャラ: zundamon, tsumugi, metan\n", char.Name)
		return
	}
	switchChar(key)
}

func handleCommand(mode *inputMode, line string) bool {
	switch line {
	case ":text":
		*mode = modeText
		fmt.Printf("モード変更: %s\n", *mode)
		return true
	case ":ptt":
		*mode = modePTT
		fmt.Printf("モード変更: %s\n", *mode)
		return true
	case ":vad":
		*mode = modeVAD
		fmt.Printf("モード変更: %s\n", *mode)
		return true
	case ":mode":
		fmt.Printf("現在のモード: %s\n", *mode)
		return true
	case ":quit", ":q":
		fmt.Println("またね！")
		os.Exit(0)
	}
	return false
}

// processText は Claude にテキストを送り、レスポンスを合成・再生する。
// パイプライン: Claude ストリーム → VoiceVox 合成 → 再生 が並列で動く。
// 戻り値の string はキャラクターの返答全文（感情タグなし）。
func processText(claudeClient *claude.Client, voicevoxClient *voicevox.Client, charName, text string) (string, error) {
	fmt.Printf("Claude に送信: %s\n", text)
	fmt.Printf("%s: ", charName)

	sentenceCh := make(chan sentenceEvent, 8)
	wavCh := make(chan []byte, 4)
	streamErrCh := make(chan error, 1)
	synthErrCh := make(chan error, 1)
	playErrCh := make(chan error, 1)

	var fullResponse strings.Builder

	// ステージ1: Claude ストリームを文単位に分割して sentenceCh に送る
	go func() {
		err := claudeClient.ChatStream(text, func(sentence, emotion string) error {
			fmt.Print(sentence)
			fullResponse.WriteString(sentence)
			sentenceCh <- sentenceEvent{text: sentence, emotion: emotion}
			return nil
		})
		close(sentenceCh)
		fmt.Println()
		streamErrCh <- err
	}()

	// ステージ2: 文を WAV に合成して wavCh に送る
	go func() {
		var firstErr error
		for se := range sentenceCh {
			if firstErr != nil {
				continue
			}
			wav, err := voicevoxClient.Synthesize(se.text, se.emotion)
			if err != nil {
				firstErr = err
				continue
			}
			wavCh <- wav
		}
		close(wavCh)
		synthErrCh <- firstErr
	}()

	// ステージ3: WAV を順番に再生する
	go func() {
		var firstErr error
		for wav := range wavCh {
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

	if streamErr != nil {
		return "", fmt.Errorf("Claude API: %w", streamErr)
	}
	if synthErr != nil {
		return "", fmt.Errorf("VOICEVOX: %w", synthErr)
	}
	if playErr != nil {
		return "", fmt.Errorf("再生エラー: %w", playErr)
	}
	return fullResponse.String(), nil
}

func processVoice(claudeClient *claude.Client, voicevoxClient *voicevox.Client, whisperClient *stt.Client, charName string, wavData []byte) error {
	fmt.Println("文字起こし中...")

	text, err := whisperClient.Transcribe(wavData)
	if err != nil {
		return fmt.Errorf("Whisper: %w", err)
	}

	if text == "" {
		fmt.Println("（音声が認識されませんでした）")
		return nil
	}

	fmt.Printf("あなた: %s\n", text)
	_, err = processText(claudeClient, voicevoxClient, charName, text)
	return err
}
