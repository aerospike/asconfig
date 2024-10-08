# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Variables required for this Makefile
ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
VERSION = $(shell git describe --tags --always)
GO_ENV_VARS =
INSTALL_DIR = /usr/local/bin
TESTDATA_DIR = $(ROOT_DIR)/testdata
COVERAGE_DIR = $(TESTDATA_DIR)/coverage
COV_UNIT_DIR = $(COVERAGE_DIR)/unit
COV_INTEGRATION_DIR = $(COVERAGE_DIR)/integration
ASCONFIG = asconfig
BUILD_DIR = $(ROOT_DIR)/bin
ACONFIG_BIN = $(BUILD_DIR)/$(ASCONFIG)

ifdef GOOS
GO_ENV_VARS = GOOS=$(GOOS)
endif

ifdef GOARCH
GO_ENV_VARS += GOARCH=$(GOARCH)
endif

SOURCES := $(shell find . -name "*.go")

# Builds asconfig binary
$(ACONFIG_BIN): $(SOURCES)
	$(GO_ENV_VARS) go build -ldflags="-X 'github.com/aerospike/asconfig/cmd.VERSION=$(VERSION)'" -o $(ACONFIG_BIN) .

GOLANGCI_LINT ?= $(GOBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.59.1

.PHONY: golanci-lint
golanci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(GOBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

go-lint: golanci-lint ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run -c .golangci.yml --cache

# Clean up
.PHONY: clean
clean:
	$(RM) bin/*
	$(RM) -r $(COVERAGE_DIR)/*
	$(RM) -r $(TESTDATA_DIR)/bin/*
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

PHONY: install
install: $(ACONFIG_BIN)
	install -m 755 $(ACONFIG_BIN) $(INSTALL_DIR)

PHONY: uninstall
uninstall:
	rm $(INSTALL_DIR)/$(ASCONFIG)

# fpm is needed to build these artifacts
.PHONY: all
all: deb rpm tar

.PHONY: deb
deb: asconfig
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: rpm
rpm: asconfig
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: tar
tar: asconfig
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: test
test: integration unit

.PHONY: integration
integration:
	mkdir -p $(COV_INTEGRATION_DIR) || true
	go test -tags=integration -timeout 30m

.PHONY: unit
unit:
	mkdir -p $(COV_UNIT_DIR) || true
	go test -tags=unit -cover ./... -args -test.gocoverdir=$(COV_UNIT_DIR)

.PHONY: coverage
coverage: test
	go tool covdata textfmt -i="$(COV_INTEGRATION_DIR),$(COV_UNIT_DIR)" -o=$(COVERAGE_DIR)/total.cov

PHONY: view-coverage
view-coverage: coverage
	go tool cover -html=$(COVERAGE_DIR)/total.cov
