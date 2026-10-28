.PHONY: run build test vet fmt tidy docker-up docker-down

run:
	go run ./cmd/api

build:
	go build -trimpath -o bin/deepsi-api ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
