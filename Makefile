.PHONY: test vet fmt lint build run docker-up docker-down migrate-check

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./...

build:
	go build -o bin/veille ./cmd/veille

run: build
	./bin/veille

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

migrate-check:
	go test ./internal/migrate/...
