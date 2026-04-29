# zun-talk

ずんだもんの声でClaudeと会話するGo製CLIアプリ。

## 必要なもの

- Go 1.22+
- [VOICEVOX](https://voicevox.hiroshiba.jp/)（起動すると `localhost:50021` でAPIサーバーが立ち上がる）
- portaudio / whisper-cpp

```bash
brew install portaudio whisper-cpp
```

## セットアップ

```bash
git clone https://github.com/min-legomain/zun-talk.git
cd zun-talk
go mod tidy
go build -o zun-talk .
```

## 実行

VOICEVOXを起動してから実行する。

```bash
export ANTHROPIC_API_KEY=sk-ant-xxxxxxxx
export WHISPER_MODEL=~/whisper.cpp/models/ggml-medium.bin

./zun-talk
```

## 入力モード

実行中に切り替え可能。

| コマンド | モード | 動作 |
|---|---|---|
| `:text` | テキスト入力 | キーボードで入力（デフォルト） |
| `:ptt` | Push-to-talk | Enterを押している間だけ録音 |
| `:vad` | 音声自動検出 | 話し終わりを自動検出して送信 |
| `:quit` | 終了 | |

## 環境変数

| 変数名 | デフォルト | 説明 |
|---|---|---|
| `ANTHROPIC_API_KEY` | なし（必須） | Anthropic APIキー |
| `WHISPER_BINARY` | `./whisper-cpp` | whisper-cppのパス |
| `WHISPER_MODEL` | `./models/ggml-medium.bin` | モデルファイルのパス |
| `VOICEVOX_URL` | `http://localhost:50021` | VOICEVOXのエンドポイント |
