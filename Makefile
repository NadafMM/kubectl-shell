# Packaging entrypoints for kubectl-shell.
# Version of truth: ./VERSION (package.json and release tags must match).

VERSION := $(shell tr -d ' \n\r\t' < VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)
GO_PACKAGES := ./cmd/kube ./internal/kube

.PHONY: all build test install dist clean version check-version

all: build

version:
	@echo $(VERSION)

check-version:
	@node -e "const fs=require('fs'); const v=fs.readFileSync('VERSION','utf8').trim(); const p=JSON.parse(fs.readFileSync('package.json','utf8')).version; if (v!==p) { console.error('VERSION ('+v+') != package.json ('+p+')'); process.exit(1); }"

build: check-version
	mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/kube ./cmd/kube

test:
	go test $(GO_PACKAGES)

install: check-version
	go install -trimpath -ldflags="$(LDFLAGS)" ./cmd/kube

dist: check-version
	node scripts/packaging/package-release.mjs

clean:
	rm -rf bin/kube dist/kubectl-shell-*.tar.gz
