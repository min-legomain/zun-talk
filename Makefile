BIN     := zun-talk
BIN_CLI := zun-talk-cli
VERSION := $(shell git describe --tags --always --dirty)
DIST    := dist

.PHONY: build build-cli run run-cli dev package clean

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

package: build
	mkdir -p $(DIST)
	hdiutil create -volname "$(BIN)" \
		-srcfolder "build/bin/$(BIN).app" \
		-ov -format UDZO \
		"$(DIST)/$(BIN)-$(VERSION).dmg"
	@echo "✓ $(DIST)/$(BIN)-$(VERSION).dmg"

clean:
	rm -f $(BIN) $(BIN_CLI)
	rm -rf frontend/dist $(DIST)
