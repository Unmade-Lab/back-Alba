.PHONY: build run dev tidy docker-up docker-down test migrate

BINARY   = alba-crm
MAIN     = ./cmd/server/main.go
BIN_DIR  = ./bin

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY) $(MAIN)

run: build
	./$(BIN_DIR)/$(BINARY)

dev:
	go run $(MAIN)

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down -v

test:
	go test -v -race ./...

migrate:
	go run $(MAIN) --migrate

.DEFAULT_GOAL := dev
