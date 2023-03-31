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
	$(GO_ENV_VARS) go build -ldflags="-X 'aerospike/asconfig/cmd.VERSION=$(VERSION)'" -o bin/asconfig .

# Clean up
.PHONY: clean
clean:
	$(RM) bin/*
	$(RM) -r testdata/coverage/*
	$(RM) -r testdata/bin/*
	$(MAKE) -C $(ROOT_DIR)/pkg/ $@

PHONY: dependencies
dependencies:
	go get github.com/wadey/gocovmerge
	go install github.com/wadey/gocovmerge

PHONY: install
install:
	install -m 755 ./bin/asconfig $(INSTALL_DIR)

PHONY: uninstall
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

PHONY: integration
integration:
	mkdir testdata/coverage/integration || true
	go test -tags=integration

	mkdir testdata/coverage/tmp_merged
	go tool covdata merge -i=testdata/coverage/integration -o=testdata/coverage/tmp_merged
	
	go tool covdata textfmt -i=testdata/coverage/tmp_merged -o=testdata/coverage/integration.cov
	rm -r testdata/coverage/tmp_merged
	rm -r testdata/coverage/integration

PHONY: unit
unit:
	mkdir testdata/coverage || true
	go test ./... -coverprofile testdata/coverage/unit.cov -coverpkg ./... -tags=unit

PHONY: coverage
coverage: dependencies integration unit
	gocovmerge testdata/coverage/*.cov > testdata/coverage/total.cov

PHONY: view-coverage
view-coverage: coverage
	go tool cover -html=testdata/coverage/total.cov
