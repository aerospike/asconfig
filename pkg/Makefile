# Variables required for this Makefile
PKG_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOP_DIR = $(PKG_DIR)/../bin
BUILD_DIR = $(PKG_DIR)/build
TARGET_DIR = $(PKG_DIR)/target
# asconfig doesn't support configs yet but fpm complains without this
CONFIG_DIR = /etc/aerospike

# Package variables
BUILD_DISTRO ?= $(shell $(PKG_DIR)/../.github/bin/os_version.sh)
NAME = "aerospike-asconfig"
MAINTAINER = "Aerospike"
VERSION ?= $(shell git describe --tags --always --abbrev=9)
RPM_VERSION := $(subst -,_,$(VERSION))
# rpm Release / deb revision. The deb revision embeds the distro per Debian
# convention (e.g. 5.0.3-1debian12), like the rpm dist tag.
ITERATION ?= 1
DEB_ITERATION = $(ITERATION)$(BUILD_DISTRO)
# pkgbuild --version rejects hyphens and underscores; collapse both to dots
# so SemVer pre-release tags (e.g. 0.20.1-rc1 -> 0.20.1.rc1) are accepted, and
# defensively normalize any underscore-form inputs (env overrides) too.
MAC_VERSION := $(subst _,.,$(subst -,.,$(VERSION)))
MAC_ROOT = $(BUILD_DIR)/mac-root
PKG_ID = com.aerospike.asconfig
MAC_PKG = $(TARGET_DIR)/aerospike-asconfig-$(MAC_VERSION)-$(ARCH).pkg
DESCRIPTION = "Asconfig, a tool for managing Aerospike database configuration files."
LICENSE = "Apache License 2.0"
URL = "https://github.com/aerospike/asconfig"
VENDOR = "Aerospike, Inc."
ARCH = $(shell uname -m)


.PHONY: all
all: deb rpm tar

# The aerospike-tools bundle ships these same /opt/aerospike/bin paths, so
# the two packages cannot coexist while it does. Two conflicts express that
# without predicting the bundle's future: the versioned one covers every
# bundle released before the bundle started declaring the virtual token (a
# frozen fact about the past, never needs raising), and the token one covers
# every future file-shipping bundle at any version (the bundle Provides it as
# long as it ships binaries, and drops it if it ever stops, at which point
# coexistence starts working with no change here). Installing either side
# over the other refuses with a named, actionable conflict instead of a raw
# file collision. Keep both values in step with citrusleaf/aerospike-tools
# and the other standalone tool repos.
TOOLS_PRE_TOKEN_BOUND = 13.0.3
TOOLS_BUNDLE_TOKEN = aerospike-tools-binaries

.PHONY: deb
deb: prep
	fpm --force \
		--config-files $(CONFIG_DIR) \
		--input-type dir \
		--output-type deb \
		--chdir $(BUILD_DIR)/ \
		--name $(NAME) \
		--version $(VERSION) \
		--iteration $(DEB_ITERATION) \
		--maintainer $(MAINTAINER) \
		--description $(DESCRIPTION) \
		--license $(LICENSE) \
		--url $(URL) \
		--vendor $(VENDOR) \
		--conflicts "aerospike-tools (<< $(TOOLS_PRE_TOKEN_BOUND))" \
		--conflicts "$(TOOLS_BUNDLE_TOKEN)" \
		--package $(TARGET_DIR)/$(NAME)_$(VERSION)-$(DEB_ITERATION)_$(ARCH).deb

.PHONY: rpm
rpm: prep
	fpm --force \
		--config-files $(CONFIG_DIR) \
		--input-type dir \
		--output-type rpm \
		--chdir $(BUILD_DIR)/ \
		--name $(NAME) \
		--version $(RPM_VERSION) \
		--iteration $(ITERATION) \
		--maintainer $(MAINTAINER) \
		--description $(DESCRIPTION) \
		--license $(LICENSE) \
		--url $(URL) \
		--vendor $(VENDOR) \
		--conflicts "aerospike-tools < $(TOOLS_PRE_TOKEN_BOUND)" \
		--conflicts "$(TOOLS_BUNDLE_TOKEN)" \
		--package $(TARGET_DIR)/$(NAME)-$(RPM_VERSION)-$(ITERATION).$(BUILD_DISTRO).$(ARCH).rpm

.PHONY: tar
tar: prep
	fpm --force \
		--config-files $(CONFIG_DIR) \
		--input-type dir \
		--output-type tar \
		--chdir $(BUILD_DIR)/ \
		--name $(NAME) \
		--version $(VERSION) \
		--maintainer $(MAINTAINER) \
		--description $(DESCRIPTION) \
		--license $(LICENSE) \
		--url $(URL) \
		--vendor $(VENDOR) \
		--package $(TARGET_DIR)/$(NAME)-$(BUILD_DISTRO)-$(VERSION)-$(ARCH).tar

.PHONY: prep
prep:
	install -d $(TARGET_DIR)
	install -d $(BUILD_DIR)/$(CONFIG_DIR)
	install -d $(BUILD_DIR)/opt/aerospike/bin
	install -pm 755 $(TOP_DIR)/asconfig $(BUILD_DIR)/opt/aerospike/bin
	install -d $(BUILD_DIR)/usr/bin
	ln -sf /opt/aerospike/bin/asconfig $(BUILD_DIR)/usr/bin/asconfig

# Staging for macOS installer: install to /usr/local/aerospike/bin with symlink at /usr/local/bin.
.PHONY: prep-mac
prep-mac:
	install -d $(TARGET_DIR)
	rm -rf $(MAC_ROOT)
	install -d $(MAC_ROOT)/usr/local/aerospike/bin
	install -d $(MAC_ROOT)/usr/local/bin
	install -pm 755 $(TOP_DIR)/asconfig $(MAC_ROOT)/usr/local/aerospike/bin/asconfig
	ln -sf ../aerospike/bin/asconfig $(MAC_ROOT)/usr/local/bin/asconfig

.PHONY: osx-pkg
osx-pkg: prep-mac
	pkgbuild \
		--root $(MAC_ROOT) \
		--identifier $(PKG_ID) \
		--version $(MAC_VERSION) \
		--install-location / \
		$(MAC_PKG)

.PHONY: clean
clean:
	rm -rf $(TARGET_DIR)
	rm -rf $(BUILD_DIR)/opt
	rm -rf $(BUILD_DIR)/var
	rm -rf $(MAC_ROOT)