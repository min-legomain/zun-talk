BIN     := zun-talk
BIN_CLI := zun-talk-cli

.PHONY: build build-cli run run-cli dev clean

# GUI モード (Wails) のプロダクションビルド
build:
	wails build -o $(BIN)

# CLI モードのビルド
build-cli:
	go build -tags cli -o $(BIN_CLI) .

# GUI モードの開発サーバー起動（ホットリロード対応）
dev:
	wails dev

# CLI モードで起動
run-cli: build-cli
	./$(BIN_CLI)

clean:
	rm -f $(BIN) $(BIN_CLI)
	rm -rf frontend/dist
