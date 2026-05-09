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

PORTAUDIO_DYLIB := $(shell brew --prefix portaudio 2>/dev/null)/lib/libportaudio.2.dylib
APP_BUNDLE     := build/bin/$(BIN).app
FRAMEWORKS_DIR := $(APP_BUNDLE)/Contents/Frameworks
MACOS_BIN      := $(APP_BUNDLE)/Contents/MacOS/$(BIN)

bundle-dylibs:
	@if [ ! -f "$(PORTAUDIO_DYLIB)" ]; then \
		echo "ERROR: libportaudio.2.dylib not found at $(PORTAUDIO_DYLIB)"; exit 1; fi
	mkdir -p $(FRAMEWORKS_DIR)
	cp "$(PORTAUDIO_DYLIB)" "$(FRAMEWORKS_DIR)/libportaudio.2.dylib"
	install_name_tool -change \
		"$(PORTAUDIO_DYLIB)" \
		"@executable_path/../Frameworks/libportaudio.2.dylib" \
		"$(MACOS_BIN)"
	codesign --force --deep --sign - "$(APP_BUNDLE)"
	@echo "✓ libportaudio.2.dylib bundled"

package: build bundle-dylibs
	mkdir -p $(DIST)
	hdiutil create -volname "$(BIN)" \
		-srcfolder "$(APP_BUNDLE)" \
		-ov -format UDZO \
		"$(DIST)/$(BIN)-$(VERSION).dmg"
	@echo "✓ $(DIST)/$(BIN)-$(VERSION).dmg"

clean:
	rm -f $(BIN) $(BIN_CLI)
	rm -rf frontend/dist $(DIST)
