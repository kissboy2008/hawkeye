.PHONY: all server agent frontend clean install

VERSION_SERVER ?= $(shell git describe --tags --match "v1.5*" --always --dirty 2>/dev/null || echo "dev")
VERSION_AGENT  ?= $(shell git describe --tags --match "v1.4*" --always --dirty 2>/dev/null || echo "dev")
LDFLAGS_SERVER = -s -w -X main.version=$(VERSION_SERVER)
LDFLAGS_AGENT  = -s -w -X main.version=$(VERSION_AGENT)

all: frontend server agent

# ========== Frontend ==========
frontend:
	cd web && npm install && npm run build

frontend-dev:
	cd web && npm install && npm run dev

# ========== Server (requires Go) ==========
server: frontend
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS_SERVER)" -o bin/hawkeye-server ./cmd/server/

server-dev:
	CGO_ENABLED=0 go run ./cmd/server/ --config ./configs/server.example.yaml

# ========== Agent (cross-compile) ==========
agent:
	CGO_ENABLED=0 GOARCH=amd64 go build -ldflags "$(LDFLAGS_AGENT)" -o bin/hawkeye-agent-amd64 ./cmd/agent/
	CGO_ENABLED=0 GOARCH=arm64 go build -ldflags "$(LDFLAGS_AGENT)" -o bin/hawkeye-agent-arm64 ./cmd/agent/

# ========== Install Go dependencies ==========
install:
	go mod tidy

# ========== Clean ==========
clean:
	rm -rf bin/ internal/static/dist/

# ========== Run locally (development) ==========
dev:
	cd web && npm run dev &
	CGO_ENABLED=0 go run ./cmd/server/ --config ./configs/server.example.yaml

# ========== Agent local dev ==========
agent-dev:
	CGO_ENABLED=0 go run ./cmd/agent/ --config ./configs/agent.example.yaml
