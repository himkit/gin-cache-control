GO ?= go
GOFMT ?= gofmt

.PHONY: test lint coverage fmt

test:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...

coverage:
	$(GO) test -cover ./...

fmt:
	$(GOFMT) -w *.go
