.PHONY: build build-js build-all test run dev bundle i18n-check i18n-missing clean

BINARY ?= forgec2-server

build:
	go build -o $(BINARY) ./cmd/server

build-js:
	powershell -ExecutionPolicy Bypass -File ./build_js.ps1 -SkipCSS

build-all: build-js build

test:
	go test ./...

run:
	./$(BINARY) -config config.yaml

dev:
	set FORGEC2_DEV=1&& go run ./cmd/server -config config.yaml

bundle: build-js

i18n-check:
	go run ./cmd/i18n-tool check

i18n-missing:
	go run ./cmd/i18n-tool missing

clean:
	go clean
	rm -f $(BINARY) server.exe forgec2-server.exe