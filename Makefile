GO ?= go
.PHONY: build test vet fmt
build:
	GOWORK=off $(GO) build ./...
test:
	GOWORK=off $(GO) test -race ./... -timeout 120s
vet:
	GOWORK=off $(GO) vet ./...
fmt:
	gofmt -w .
