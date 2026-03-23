.PHONY: build run test test-unit test-integration test-cover lint \
       docker-up docker-down docker-logs docker-status clean

BINARY_NAME=server
BUILD_DIR=./cmd/server

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) $(BUILD_DIR)

run: build
	./bin/$(BINARY_NAME)

test: test-unit

test-unit:
	go test -race -count=1 ./internal/handler/... ./internal/service/...

test-integration:
	INTEGRATION_TEST=true go test -race -count=1 -timeout 120s ./internal/...

test-cover:
	go test -race -coverprofile=coverage.out ./internal/handler/... ./internal/service/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

docker-up:
	docker compose up --build -d
	@echo "Waiting for healthy services..."
	@docker compose ps --format "table {{.Name}}\t{{.Status}}" 2>/dev/null || true
	@echo ""
	@echo "Run 'make docker-logs' to follow logs"
	@echo "Run 'make docker-status' to check health"

docker-down:
	docker compose down -v

docker-logs:
	docker compose logs -f

docker-status:
	@docker compose ps

clean:
	rm -rf bin/ coverage.out coverage.html
	docker compose down -v 2>/dev/null || true
