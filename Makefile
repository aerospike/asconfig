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
asconfig:
	$(GO_ENV_VARS) go build -ldflags="-X 'cmd.version=$(VERSION)'" -o asconfig .

# Clean up
.PHONY: clean
clean:
	rm asconfig
	rm -r coverage

.Phony: dependencies
dependencies:
	go get github.com/wadey/gocovmerge
	go install github.com/wadey/gocovmerge

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
