BIN := zun-talk

.PHONY: build
build:
	go build -o $(BIN) .
