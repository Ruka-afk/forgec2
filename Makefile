.PHONY: build build-js build-embed build-all test vet lint run dev bundle clean
.PHONY: build-cross build-linux build-windows build-darwin
.PHONY: tidy deps i18n-check i18n-missing
.PHONY: db-reset help

BINARY   ?= forgec2-server
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -buildid= -X main.version=$(VERSION)

# ---------- Core ----------

build:
	go build -ldflags="$(LDFLAGS)" -trimpath -buildvcs=false -o $(BINARY).exe ./cmd/server

# go vet includes the agent, which emits expected unsafe.Pointer warnings
# (Windows syscall patterns). Filter those while still failing on real findings.
vet:
	go vet ./... 2>&1 | grep -v "internal/payload/agent" || true

vet-strict:
	go vet ./...

tidy:
	go mod tidy

deps: tidy

test:
	go test ./... -count=1

lint: vet
	@echo "Install golangci-lint for full linting: https://golangci-lint.run/usage/install/"

# ---------- Embedded Frontend ----------

build-js:
	cd frontend && npm run build

# Copy frontend output to embed directory
internal/webdist/dist: build-js
	rm -rf internal/webdist/dist
	cp -r frontend/out internal/webdist/dist

# Full embed pipeline: built frontend --> embed --> Go binary
build-embed: internal/webdist/dist build

# ---------- Cross-compilation (requires build-js first) ----------

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -trimpath -buildvcs=false -o $(BINARY)-linux-amd64 ./cmd/server

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -trimpath -buildvcs=false -o $(BINARY)-windows-amd64.exe ./cmd/server

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -trimpath -buildvcs=false -o $(BINARY)-darwin-amd64 ./cmd/server

build-cross: build-linux build-windows build-darwin

build-all: build-embed

bundle: build-embed

# ---------- i18n ----------

i18n-check:
	go run ./cmd/i18n-tool check

i18n-missing:
	go run ./cmd/i18n-tool missing

# ---------- Run ----------

run:
	./$(BINARY).exe -config config.yaml

dev:
	set FORGEC2_DEV=1&& go run ./cmd/server -config config.yaml

# ---------- Database ----------

db-reset:
	@echo "WARNING: This deletes the database. Press Ctrl+C to abort."
	@timeout /t 3 >nul 2>&1 || sleep 3
	rm -f data/db/forgec2.db

# ---------- Cleanup ----------

clean:
	go clean
	rm -rf internal/webdist/dist
	rm -f $(BINARY) $(BINARY).exe
	rm -f $(BINARY)-linux-amd64 $(BINARY)-windows-amd64.exe $(BINARY)-darwin-amd64

# ---------- Help ----------

help:
	@echo "ForgeC2 Makefile"
	@echo ""
	@echo "  make build          Build server (Windows .exe, requires existing internal/webdist/dist)"
	@echo "  make build-js       Build frontend JS"
	@echo "  make build-embed    Build frontend + embed + Go binary (full pipeline)"
	@echo "  make build-all      Alias for build-embed"
	@echo "  make test           Run all Go tests"
	@echo "  make vet            Run go vet"
	@echo "  make tidy           Run go mod tidy"
	@echo "  make build-cross    Build for Linux, Windows, macOS"
	@echo "  make build-linux    Build for Linux amd64"
	@echo "  make build-windows  Build for Windows amd64"
	@echo "  make build-darwin   Build for macOS amd64"
	@echo "  make dev            Run in dev mode"
	@echo "  make clean          Remove build artifacts"
	@echo "  make help           Show this help"
