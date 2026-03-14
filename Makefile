.PHONY: build run dev clean

build:
	go build -o dist/telegram-claude-bot ./cmd/bot/

run: build
	./dist/telegram-claude-bot

dev:
	air

clean:
	rm -rf dist/

tidy:
	go mod tidy

test:
	go test ./...
