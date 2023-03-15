# Variables required for this Makefile
ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
VERSION = $(shell git describe --tags --always)
GO_ENV_VARS =
INSTALL_DIR = /usr/local/bin

ifdef GOOS
GO_ENV_VARS = GOOS=$(GOOS)
endif

ifdef GOARCH
GO_ENV_VARS += GOARCH=$(GOARCH)
endif

# Builds exporter binary
.PHONY: asconfig
asconfig:
	$(GO_ENV_VARS) go build -ldflags="-X 'aerospike/asconfig/cmd.VERSION=$(VERSION)'" -o asconfig .

# Clean up
.PHONY: clean
clean:
	$(RM) asconfig
	$(RM) -r coverage
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

.Phony: dependencies
dependencies:
	go get github.com/wadey/gocovmerge
	go install github.com/wadey/gocovmerge

.Phony: install
install:
	install -m 755 ./asconfig $(INSTALL_DIR)

.Phony: uninstall
uninstall:
	rm $(INSTALL_DIR)/asconfig

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

.Phony: integration
integration:
	mkdir coverage || true
	go test -tags=integration -coverpkg=./... -coverprofile=coverage/integration.cov

.Phony: unit
unit:
	mkdir coverage || true
	go test ./... -coverprofile coverage/unit.cov -coverpkg ./... -tags=unit

.Phony: coverage
coverage: dependencies integration unit
	gocovmerge coverage/*.cov > coverage/total.cov

.Phony: view-coverage
view-coverage: coverage
	go tool cover -html=coverage/total.cov
