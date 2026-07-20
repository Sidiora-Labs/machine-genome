SHELL := /bin/sh

GO ?= go
GOFLAGS ?= -buildvcs=false
BINARY ?= bin/mgs
VERSION := $(shell sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' mgs/record.go)
COMMIT := $(shell test -e .git && git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.buildVersion=$(VERSION) -X main.buildCommit=$(COMMIT)
DIST_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all help bootstrap-check build install test race coverage vet fmt-check \
	mod-verify schemas api records docs repo-check check dist release-check clean

all: check build

help:
	@printf '%s\n' \
	  'Machine Genome development targets:' \
	  '  make check          complete local conformance and repository gate' \
	  '  make race           run all tests with the race detector' \
	  '  make build          build a static local mgs binary' \
	  '  make coverage       write coverage.out and coverage.html' \
	  '  make dist           build cross-platform archives and checksums' \
	  '  make release-check  validate VERSION=x.y.z from a clean tree' \
	  '  make clean          remove generated build and coverage artifacts'

bootstrap-check:
	@command -v $(GO) >/dev/null || { echo "Go is required"; exit 1; }
	@command -v jq >/dev/null || { echo "jq is required"; exit 1; }
	@command -v python3 >/dev/null || { echo "Python 3 is required"; exit 1; }
	@$(GO) version
	@jq --version
	@python3 --version

build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/mgs

install:
	CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" $(GO) install -trimpath -ldflags "$(LDFLAGS)" ./cmd/mgs

test:
	GOFLAGS="$(GOFLAGS)" $(GO) test ./...

race:
	CGO_ENABLED=1 GOFLAGS="$(GOFLAGS)" $(GO) test -race ./...

coverage:
	GOFLAGS="$(GOFLAGS)" $(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

vet:
	GOFLAGS="$(GOFLAGS)" $(GO) vet ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd mgs registry -name '*.go' -type f))" || \
	  { echo "Go files require gofmt"; gofmt -l $$(find cmd mgs registry -name '*.go' -type f); exit 1; }
	@if rg -n '[[:blank:]]+$$' \
	  --glob '!go.sum' --glob '!records/**/*.json' \
	  --glob '*.go' --glob '*.md' --glob '*.json' --glob '*.yml' \
	  --glob '*.yaml' --glob 'Dockerfile' .; then \
	  echo "trailing whitespace found"; exit 1; \
	fi
	git diff --check

mod-verify:
	$(GO) mod verify

schemas:
	@for schema in schema/*.json; do jq -e . "$$schema" >/dev/null; done

api:
	jq -e '.openapi == "3.1.0" and .info.title == "Machine Genome Registry API"' api/openapi.json >/dev/null

records: build
	./scripts/verify-records.sh $(BINARY)

docs:
	@for script in scripts/*.sh; do sh -n "$$script"; done
	python3 -c 'import ast, pathlib; ast.parse(pathlib.Path("scripts/check-repository.py").read_text())'
	python3 ./scripts/check-repository.py

repo-check: schemas api docs fmt-check

check: test vet mod-verify repo-check records

dist: clean
	@set -eu; \
	for target in $(DIST_TARGETS); do \
	  os=$${target%/*}; arch=$${target#*/}; ext=""; \
	  if [ "$$os" = windows ]; then ext=".exe"; fi; \
	  name="machine-genome_$(VERSION)_$${os}_$${arch}"; \
	  mkdir -p "dist/$$name"; \
	  GOOS="$$os" GOARCH="$$arch" CGO_ENABLED=0 GOFLAGS="$(GOFLAGS)" \
	    $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
	    -o "dist/$$name/mgs$$ext" ./cmd/mgs; \
	  cp LICENSE NOTICE README.md "dist/$$name/"; \
	  tar -C dist -czf "dist/$$name.tar.gz" "$$name"; \
	  rm -rf "dist/$$name"; \
	done
	tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
	  --exclude='./.git' --exclude='./bin' --exclude='./dist' \
	  --exclude='./.local' --exclude='./coverage.out' --exclude='./coverage.html' \
	  --exclude='*.key.json' --exclude='admin.token' \
	  --exclude='*.db' --exclude='*.backup.db' \
	  --transform='s,^\.,machine-genome_$(VERSION)_source,' \
	  -czf "dist/machine-genome_$(VERSION)_source.tar.gz" .
	cd dist && sha256sum *.tar.gz > checksums.txt

release-check:
	@test -n "$(VERSION)" || { echo "usage: make release-check VERSION=x.y.z"; exit 1; }
	./scripts/release-check.sh "$(VERSION)"

clean:
	rm -rf bin dist coverage.out coverage.html
