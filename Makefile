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
all: $(ACONFIG_BIN) deb rpm tar

.PHONY: deb
deb: $(ACONFIG_BIN)
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: rpm
rpm: $(ACONFIG_BIN)
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: tar
tar: $(ACONFIG_BIN)
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  install-golangci-lint  Install golangci-lint v2.4.0"
	@echo "  check-golangci-lint    Check if golangci-lint is installed"
	@echo "  go-lint                Run golangci-lint (auto-installs if needed)"
	@echo "  go-lint-fix            Run golangci-lint with --fix (auto-installs if needed)"
	@echo "  test                   Run unit and integration tests"
	@echo "  unit                   Run unit tests"
	@echo "  integration            Run integration tests"
	@echo "  coverage               Generate coverage report"
	@echo "  view-coverage          View coverage report in browser"
	@echo "  clean                  Clean build artifacts"
	@echo "  install                Install asconfig to $(INSTALL_DIR)"
	@echo "  uninstall              Remove asconfig from $(INSTALL_DIR)"

.PHONY: test
test: integration unit

.PHONY: install-golangci-lint
install-golangci-lint:
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.4.0
	@echo "golangci-lint installed successfully!"
	@golangci-lint --version

.PHONY: check-golangci-lint
check-golangci-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-golangci-lint' to install it." && exit 1)

.PHONY: go-lint
go-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && $(MAKE) install-golangci-lint)
	golangci-lint run --config .golangci.yml

.PHONY: go-lint-fix
go-lint-fix:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && $(MAKE) install-golangci-lint)
	golangci-lint run --config .golangci.yml --fix

# Keep old format targets for backward compatibility
.PHONY: format
format: go-lint

.PHONY: format-fix
format-fix: go-lint-fix

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
