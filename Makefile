VERSION   := $(shell cat VERSION)
BIN       := twemproxy_exporter
CONTAINER := twemproxy_exporter
GOOS      ?= linux
GOARCH    ?= amd64
PREFIX    ?= $(shell pwd)
BINDIR    ?= $(shell pwd)

default: $(BIN)

$(BIN): $(wildcard *.go) go.mod go.sum
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) promu build $@ --prefix $(PREFIX)

tarball: $(BIN)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) promu tarball $(BINDIR) --prefix $(PREFIX)
