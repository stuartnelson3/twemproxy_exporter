VERSION   := $(shell cat VERSION)
BIN       := twemproxy_exporter
CONTAINER := twemproxy_exporter
GOOS      ?= linux
GOARCH    ?= amd64
PREFIX    ?= $(shell pwd)
BINDIR    ?= $(shell pwd)

default: $(BIN)

$(BIN): $(wildcard *.go) go.mod go.sum promu
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(PROMU) build $@ --prefix $(PREFIX)

tarball: $(BIN)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(PROMU) tarball $(BINDIR) --prefix $(PREFIX)

GO                ?= go
GO_VERSION        ?= $(shell $(GO) version)
GO_BUILD_PLATFORM ?= $(subst /,-,$(lastword $(GO_VERSION)))
FIRST_GOPATH      := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))

PROMU_VERSION ?= 0.2.0
PROMU_URL     := https://github.com/prometheus/promu/releases/download/v$(PROMU_VERSION)/promu-$(PROMU_VERSION).$(GO_BUILD_PLATFORM).tar.gz
PROMU         := $(FIRST_GOPATH)/bin/promu

.PHONY: promu
promu: $(PROMU)

$(PROMU):
	curl -s -L $(PROMU_URL) | tar -xvz -C /tmp
	mkdir -v -p $(FIRST_GOPATH)/bin
	cp -v /tmp/promu-$(PROMU_VERSION).$(GO_BUILD_PLATFORM)/promu $(PROMU)
