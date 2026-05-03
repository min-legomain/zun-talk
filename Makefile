BIN := zun-talk

.PHONY: build run
build:
	go build -o $(BIN) .

run: build
	./$(BIN)
