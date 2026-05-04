# zun-talk UI 設計書

## 概要

CLI ベースの `zun-talk` を [Wails](https://wails.io/) + React によるデスクトップアプリへ移行する。  
バックエンドの Go コードはほぼそのまま流用し、Wails がブリッジ層となって React フロントエンドと接続する。

---

## 1. 技術スタック

| 層 | 技術 |
|---|---|
| デスクトップシェル | Wails v2 |
| バックエンド | Go 1.21+（既存コードを流用） |
| フロントエンド | React 18 + TypeScript |
| スタイリング | Tailwind CSS |
| 状態管理 | Zustand |
| ビルドツール | Vite（Wails 標準） |

---

## 2. ディレクトリ構成（移行後）

```
zun-talk/
├── main.go                  # Wails エントリポイント（既存 main.go を置き換え）
├── app.go                   # Wails App 本体（Go メソッドをフロントエンドに公開）
├── claude/client.go         # 変更なし
├── config/                  # 変更なし
├── player/player.go         # 変更なし
├── recorder/recorder.go     # 変更なし
├── stt/whisper.go           # 変更なし
├── voicevox/client.go       # 変更なし
├── wailsjs/                 # Wails が自動生成するバインディング
│   └── go/main/
│       └── App.js / App.d.ts
└── frontend/
    ├── index.html
    ├── vite.config.ts
    ├── src/
    │   ├── main.tsx
    │   ├── App.tsx
    │   ├── store/
    │   │   └── chatStore.ts       # Zustand ストア
    │   ├── components/
    │   │   ├── ChatWindow.tsx     # 会話ログ表示
    │   │   ├── MessageBubble.tsx  # 1 件のメッセージ
    │   │   ├── InputBar.tsx       # テキスト入力 + 送信
    │   │   ├── VoiceButton.tsx    # PTT / VAD 操作ボタン
    │   │   ├── CharacterPanel.tsx # キャラクター選択
    │   │   ├── StatusBar.tsx      # 状態表示（録音中、合成中…）
    │   │   └── DuetPanel.tsx      # デュエットモード UI
    │   └── types/
    │       └── index.ts           # 共有型定義
    └── public/
        └── characters/            # キャラクターアイコン画像
```

---

## 3. Wails バインディング設計（app.go）

React から呼び出せる Go メソッドを `app.go` に定義する。

### 3.1 公開メソッド一覧

```go
// キャラクター
func (a *App) GetCharacters() []CharacterInfo
func (a *App) SetCharacter(name string) error

// 会話
func (a *App) SendMessage(text string) error
func (a *App) GetHistory() []Message

// 入力モード
func (a *App) SetInputMode(mode string) error  // "text" | "ptt" | "vad"
func (a *App) StartRecording() error            // PTT 開始
func (a *App) StopRecording() error             // PTT 終了

// デュエット
func (a *App) StartDuet(char1 string, char2 string) error
func (a *App) StopDuet() error

// トピック提案
func (a *App) GetTopics() ([]string, error)
```

### 3.2 Wails イベント（Go → React）

Go 側から `runtime.EventsEmit` で送出し、React 側が `EventsOn` で受信する。

| イベント名 | ペイロード | 説明 |
|---|---|---|
| `message:sentence` | `{text, emotion, speaker}` | 1 文ずつストリームされる |
| `message:done` | `{fullText}` | 1 ターン完了 |
| `status:changed` | `{status}` | 状態変化（録音中、合成中、再生中…） |
| `recording:transcribed` | `{text}` | 音声認識結果 |
| `duet:sentence` | `{text, emotion, speaker}` | デュエット中の 1 文 |
| `error:occurred` | `{message}` | エラー通知 |

### 3.3 型定義（Go 側、自動的に TypeScript 型へ変換される）

```go
type CharacterInfo struct {
    Name      string `json:"name"`
    SpeakerID int    `json:"speakerId"`
    IconPath  string `json:"iconPath"`
}

type Message struct {
    ID        string    `json:"id"`
    Role      string    `json:"role"`   // "user" | "assistant"
    Text      string    `json:"text"`
    Emotion   string    `json:"emotion"`
    Speaker   string    `json:"speaker"`
    Timestamp time.Time `json:"timestamp"`
}
```

---

## 4. 画面設計

### 4.1 全体レイアウト

```
┌─────────────────────────────────────────────────────────────────┐
│  [ずんだもん ▼]  [めたん]  [つむぎ]          [デュエット] [設定]  │  ← ヘッダー
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  😊 ずんだもん                                            │  │
│   │  「こんにちは！ずんだもんなのだ！」                        │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│                   ┌──────────────────────────────────────────┐  │
│                   │  あなた                                   │  │
│                   │  「今日の天気は？」                        │  │
│                   └──────────────────────────────────────────┘  │
│                                                                  │  ← チャットウィンドウ
│   ┌──────────────────────────────────────────────────────────┐  │
│   │  😢 ずんだもん                             ♪ 再生中...   │  │
│   │  「今日は雨なのだ... 少し悲しいのだ...」                   │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│  [● 録音]  ┌──────────────────────────────────────┐  [送信]   │  ← 入力バー
│  [VAD ON]  │ メッセージを入力...                    │           │
│            └──────────────────────────────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│  モード: テキスト  |  キャラクター: ずんだもん  |  ステータス: 待機中 │  ← ステータスバー
└─────────────────────────────────────────────────────────────────┘
```

---

### 4.2 コンポーネント詳細

#### `CharacterPanel`（ヘッダー左）
- 選択中キャラクターをハイライト表示
- クリックでキャラクター切り替え → `SetCharacter()` 呼び出し
- キャラクターアイコン（32px）+ 名前

#### `ChatWindow`
- メッセージリストを縦スクロール
- 新規メッセージ追加時に自動スクロール
- `message:sentence` イベントで AI メッセージをリアルタイム追記（ストリーム表示）

#### `MessageBubble`
- ユーザー発言：右寄せ、グレー背景
- AI 発言：左寄せ、キャラクターカラー背景
- 感情アイコン（😊😢😮😠）を左上に表示
- 再生中メッセージは音符アイコン（♪）でインジケーター
- 感情に応じてバブル色を微妙に変化

| 感情 | アイコン | バブル色 |
|---|---|---|
| 普通 | 😐 | キャラカラー（標準） |
| 喜び | 😊 | 少し明るい |
| 悲しみ | 😢 | 少し暗い・青み |
| 驚き | 😮 | 黄みがかる |
| 怒り | 😠 | 赤み |

#### `InputBar`
- テキスト入力（Textarea、Enter で送信、Shift+Enter で改行）
- 録音ボタン（PTT：押している間録音、VAD：トグル）
- 入力モード切り替えボタン
- 送信中はボタンを無効化

#### `VoiceButton`
- PTT モード：マウスダウン/アップで `StartRecording` / `StopRecording` 呼び出し
- VAD モード：トグルスイッチ、録音中は波形アニメーション表示
- 音声認識中はスピナーを表示、認識結果をテキスト欄に自動挿入

#### `StatusBar`（フッター）
- 現在の入力モード
- 選択中キャラクター
- ステータス文字列（`status:changed` イベントで更新）

#### `DuetPanel`（モーダル or サイドパネル）
- キャラクター 1 / キャラクター 2 を選択
- 開始 / 停止ボタン
- デュエット中は両キャラのアイコンを交互にアニメーション

---

## 5. 状態管理（Zustand ストア）

```typescript
// src/store/chatStore.ts

interface ChatState {
  // キャラクター
  characters: CharacterInfo[];
  activeCharacter: string;
  setActiveCharacter: (name: string) => void;

  // 会話履歴
  messages: Message[];
  addMessage: (msg: Message) => void;
  appendToLastMessage: (text: string, emotion: string) => void;

  // 入力モード
  inputMode: 'text' | 'ptt' | 'vad';
  setInputMode: (mode: 'text' | 'ptt' | 'vad') => void;

  // 状態フラグ
  status: 'idle' | 'recording' | 'transcribing' | 'thinking' | 'speaking';
  setStatus: (status: ChatState['status']) => void;

  // デュエット
  isDuetMode: boolean;
  duetChars: [string, string] | null;
  startDuet: (char1: string, char2: string) => void;
  stopDuet: () => void;

  // エラー
  error: string | null;
  setError: (msg: string | null) => void;
}
```

---

## 6. データフロー

### 6.1 テキスト送信フロー

```
[InputBar] 送信
    │
    ├─→ store.addMessage({ role: "user", text })
    ├─→ store.setStatus("thinking")
    │
    └─→ App.SendMessage(text)  [Wails バインディング]
            │
            └─ Go: processText()
                    ├─ claude.ChatStream() ストリーム開始
                    │       │
                    │       └─ 1 文ごとに EventsEmit("message:sentence", {text, emotion})
                    │                              ↓
                    │                   [ChatWindow] appendToLastMessage()
                    │
                    ├─ voicevox.Synthesize() → player.Play()
                    │
                    └─ EventsEmit("message:done", {fullText})
                                       ↓
                            store.setStatus("idle")
```

### 6.2 PTT 録音フロー

```
[VoiceButton] mousedown
    └─→ App.StartRecording()
            └─ Go: recorder.RecordPushToTalk() 開始
                    └─ status:changed { status: "recording" }

[VoiceButton] mouseup
    └─→ App.StopRecording()
            └─ Go: 録音停止 → stt.Transcribe()
                    ├─ status:changed { status: "transcribing" }
                    └─ recording:transcribed { text }
                                    ↓
                        InputBar のテキスト欄に自動挿入
                        → 自動送信 or ユーザーが確認して送信
```

### 6.3 VAD フロー

```
[VoiceButton] VAD トグル ON
    └─→ App.SetInputMode("vad")
            └─ Go: 音声検知ループ開始
                    ├─ 音声検知 → status:changed { status: "recording" }
                    ├─ 無音検知 → stt.Transcribe()
                    │       └─ recording:transcribed { text }
                    │                   ↓
                    │       自動的に SendMessage() を呼び出し
                    └─ ループ継続
```

---

## 7. app.go 実装方針

既存の `main.go` の処理を `App` 構造体のメソッドとして切り出す。

```go
type App struct {
    ctx        context.Context
    claudeClient *claude.Client
    vvClient   *voicevox.Client
    player     *player.Player
    recorder   *recorder.Recorder
    config     *config.Config
    characters []config.Character
    activeChar *config.Character
    inputMode  string
    duetCancel context.CancelFunc
    mu         sync.Mutex
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    // 既存の初期化処理を移植
}
```

`processText()` の中で文ごとに `runtime.EventsEmit` を呼ぶことで、  
既存のコールバックベースのストリーミング処理をそのまま活用できる。

---

## 8. UI スタイル方針

- **テーマ**: ずんだもんカラー（緑 #4CAF50）をベースとしたライトテーマ
- **フォント**: Noto Sans JP（日本語）
- **キャラクターカラー**:
  - ずんだもん: `#4CAF50`（緑）
  - めたん: `#FF6B9D`（ピンク）
  - つむぎ: `#FF8C00`（オレンジ）
- **アニメーション**: Framer Motion でメッセージ追加時のスライドイン
- **ウィンドウサイズ**: デフォルト 800×600、最小 600×400

---

## 9. 実装フェーズ

| フェーズ | 内容 | 優先度 |
|---|---|---|
| Phase 1 | Wails プロジェクト初期化、app.go の骨格 | 高 |
| Phase 2 | テキスト送信 → AI レスポンス表示（音声なし） | 高 |
| Phase 3 | 音声合成・再生のストリーミング表示 | 高 |
| Phase 4 | キャラクター選択 UI | 中 |
| Phase 5 | PTT / VAD 音声入力 | 中 |
| Phase 6 | デュエットモード | 低 |
| Phase 7 | トピック提案、ヘルプ | 低 |

---

## 10. 既存 CLI との互換性

Wails 化後も CLI モード（`go run main.go --cli`）で動作させたい場合は、  
`app.go` に抽出したビジネスロジックを CLI の `main.go` からも呼び出せる設計にする。  
ただし Phase 1 では CLI 互換性は考慮せず、まずデスクトップアプリとして完成させる。
