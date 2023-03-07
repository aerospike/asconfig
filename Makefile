# Variables required for this Makefile
VERSION = $(shell git describe --tags --always)
GO_ENV_VARS =

ifdef GOOS
GO_ENV_VARS = GOOS=$(GOOS)
endif

ifdef GOARCH
GO_ENV_VARS += GOARCH=$(GOARCH)
endif

# Builds exporter binary
.PHONY: asconfig
exporter:
	$(GO_ENV_VARS) go build -ldflags="-X 'cmd.version=$(VERSION)'" -o asconfig .

# Clean up
.PHONY: clean
clean:
	rm asconfig

.PHONY: test
test: integration


.Phony: integration
integration:
	go test -v -coverpkg=./... integration_test.go