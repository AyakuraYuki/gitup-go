BIN_NAME := git-updater
GOOS     ?= darwin
GOARCH   ?= arm64

.PHONY: install-staticcheck check build

install-staticcheck:
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi

check: install-staticcheck
	@staticcheck ./...

build:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-s -w" -o ./dist/$(BIN_NAME)-$(GOOS)_$(GOARCH) .

clean:
	@go clean
	@rm -rf ./dist
