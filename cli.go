//go:build cli

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"zun-talk/claude"
	"zun-talk/config"
	"zun-talk/engine"
	"zun-talk/player"
	"zun-talk/recorder"
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

func runCLI(charKey string) {
	cfg := config.Load()

	if err := config.LoadCharacters(cfg.CharSettingsPath, cfg.ConvGuidePath); err != nil {
		fmt.Fprintf(os.Stderr, "キャラクター設定の読み込みに失敗: %v\n", err)
		os.Exit(1)
	}

	if cfg.AnthropicAPIKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY が設定されていません")
		os.Exit(1)
	}

	if err := player.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "player.Init: %v\n", err)
		os.Exit(1)
	}

	cb := engine.Callbacks{
		OnSentence: func(text, emotion, speaker string) {
			fmt.Print(text)
		},
		OnStatus: func(status string) {
			if status == "transcribing" {
				fmt.Println("文字起こし中...")
			}
		},
		OnTranscribed: func(text string) {
			fmt.Printf("あなた: %s\n", text)
		},
	}

	eng, err := engine.New(cfg, charKey, cb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エンジン初期化失敗: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("zun-talk へようこそ！（キャラ: %s）\n", eng.ActiveCharacter().Name)
	fmt.Printf("コマンド一覧は :help で確認できます\n\n")

	mode := modeText
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if mode == modeVAD {
			fmt.Println("話しかけてください | :text / :ptt / :quit を入力して切り替え")
			wavData, err := recorder.RecordVAD()
			if err != nil {
				fmt.Fprintf(os.Stderr, "録音エラー: %v\n", err)
				continue
			}
			if err := eng.ProcessVoice(wavData); err != nil {
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
			key := strings.TrimSpace(strings.TrimPrefix(line, ":char"))
			if key == "" {
				c := eng.ActiveCharacter()
				fmt.Printf("現在のキャラ: %s\n使えるキャラ: zundamon, tsumugi, metan\n", c.Name)
			} else if err := eng.SetCharacter(key); err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("キャラクター変更: %s\n", eng.ActiveCharacter().Name)
			}
			continue
		}

		if strings.HasPrefix(line, ":duet") {
			parts := strings.Fields(line)
			if len(parts) != 3 {
				fmt.Println("使い方: :duet <キャラA> <キャラB>（例: :duet zundamon metan）")
				continue
			}
			charA, okA := config.Characters[parts[1]]
			charB, okB := config.Characters[parts[2]]
			if !okA || !okB {
				fmt.Println("不明なキャラクター。使えるキャラ: zundamon, tsumugi, metan")
				continue
			}
			cfg := eng.Config()
			clientA := claude.NewClient(cfg.AnthropicAPIKey, charA.Prompt)
			clientB := claude.NewClient(cfg.AnthropicAPIKey, charB.Prompt)
			voiceA := voicevox.NewClient(cfg.VoicevoxURL, charA.SpeakerID)
			voiceB := voicevox.NewClient(cfg.VoicevoxURL, charB.SpeakerID)
			fmt.Printf("デュエット開始: %s × %s\n", charA.Name, charB.Name)
			fmt.Println("Enter で会話続ける | 文字入力で参加 | :stop で終了")
			runDuet(eng, clientA, voiceA, charA.Name, clientB, voiceB, charB.Name, scanner)
			fmt.Println("デュエット終了")
			continue
		}

		if line == ":topic" {
			const topicPrompt = "今から話せる面白い話題を3つ提案して。一言ずつで。"
			fmt.Printf("%s: ", eng.ActiveCharacter().Name)
			if _, err := eng.ProcessText(topicPrompt); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
			fmt.Println()
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
			if err := eng.ProcessVoice(wavData); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
		} else {
			fmt.Printf("%s: ", eng.ActiveCharacter().Name)
			if _, err := eng.ProcessText(line); err != nil {
				fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			}
			fmt.Println()
		}
	}
}

func printHelp() {
	fmt.Println("=== コマンド一覧 ===")
	fmt.Println(":text / :ptt / :vad      入力モード切替")
	fmt.Println(":char <名前>             キャラ切替 (zundamon / tsumugi / metan)")
	fmt.Println(":duet <キャラA> <キャラB>  デュエット会話")
	fmt.Println(":topic                  話題を3つ提案")
	fmt.Println(":help                   このヘルプを表示")
	fmt.Println(":quit                   終了")
	fmt.Println("===================")
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
	case ":help":
		printHelp()
		return true
	case ":quit", ":q":
		fmt.Println("またね！")
		os.Exit(0)
	}
	return false
}

func runDuet(
	eng *engine.Engine,
	clientA *claude.Client, voiceA *voicevox.Client, nameA string,
	clientB *claude.Client, voiceB *voicevox.Client, nameB string,
	scanner *bufio.Scanner,
) {
	fmt.Printf("%s: ", nameA)
	lastA, err := eng.ProcessTextWith(clientA, voiceA, nameA,
		fmt.Sprintf("%sと楽しい会話を始めてください。", nameB))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		return
	}

	fmt.Printf("%s: ", nameB)
	lastB, err := eng.ProcessTextWith(clientB, voiceB, nameB,
		fmt.Sprintf("%sが「%s」と言いました。", nameA, lastA))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		return
	}

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())

		switch line {
		case ":stop", ":text", ":ptt", ":vad":
			return
		case ":quit", ":q":
			fmt.Println("またね！")
			os.Exit(0)
		case ":help":
			fmt.Println("デュエット中: Enter で続ける | 文字入力で参加 | :stop で終了")
			continue
		}
		if strings.HasPrefix(line, ":") {
			fmt.Println("デュエット中は使えないコマンドです。:stop でデュエット終了")
			continue
		}

		var msgA string
		if line != "" {
			fmt.Printf("あなた: %s\n", line)
			msgA = fmt.Sprintf("ユーザーが「%s」と言いました。%sも「%s」と言っています。反応してください。", line, nameB, lastB)
		} else {
			msgA = fmt.Sprintf("%sが「%s」と言いました。", nameB, lastB)
		}

		fmt.Printf("%s: ", nameA)
		lastA, err = eng.ProcessTextWith(clientA, voiceA, nameA, msgA)
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
			continue
		}

		var msgB string
		if line != "" {
			msgB = fmt.Sprintf("ユーザーが「%s」と言い、%sが「%s」と返しました。あなたも反応してください。", line, nameA, lastA)
		} else {
			msgB = fmt.Sprintf("%sが「%s」と言いました。", nameA, lastA)
		}

		fmt.Printf("%s: ", nameB)
		lastB, err = eng.ProcessTextWith(clientB, voiceB, nameB, msgB)
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		}
	}
}
