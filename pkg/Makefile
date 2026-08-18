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
# Package names never carry the "rcN" pre-release tag: rcN becomes the package
# iteration, so 0.21.3-rc2 packages as version 0.21.3 iteration 2.
PKG_RELEASE_SH = $(PKG_DIR)/../.github/bin/pkg_release.sh
PKG_VERSION := $(shell $(PKG_RELEASE_SH) '$(VERSION)' version)
ITERATION ?= $(shell $(PKG_RELEASE_SH) '$(VERSION)' iteration)
# For names with no iteration field of their own (.tar, macOS .pkg).
RELEASE := $(PKG_VERSION)-$(ITERATION)
RPM_VERSION := $(subst -,_,$(PKG_VERSION))
# rpm Release / deb revision. The deb revision embeds the distro per Debian
# convention (e.g. 0.20.1-1debian12), like the rpm dist tag.
DEB_ITERATION = $(ITERATION)$(BUILD_DISTRO)
# pkgbuild --version rejects hyphens and underscores; collapse both to dots
# (0.20.1-1 -> 0.20.1.1), and defensively normalize any underscore-form
# inputs (env overrides) too. This is the version *inside* the pkg only;
# the file name below keeps the canonical hyphenated RELEASE.
MAC_VERSION = $(subst _,.,$(subst -,.,$(RELEASE)))
MAC_ROOT = $(BUILD_DIR)/mac-root
PKG_ID = com.aerospike.asconfig
# macOS artifact file name:
#
#     aerospike-asconfig-<version>-<iteration>-macos<major>-<arch>.pkg
#     e.g. aerospike-asconfig-0.20.1-1-macos15-arm64.pkg
#
# Lower case and '-' separated so a Homebrew cask can interpolate it directly
# (`arch arm: "arm64", intel: "x86_64"` + url ".../...-macos15-#{arch}.pkg").
# <arch> is `uname -m` -- the tokens Homebrew's Hardware::CPU.arch emits, not
# runner.arch (ARM64 / X64: upper case, and "X64" is not a macOS arch).
# macos<major> is the build host's macOS major version -- the oldest macOS the
# artifact supports, and what keeps names unique when one version ships a pkg
# per (macOS major, arch) pair. The version field is RELEASE (version +
# iteration), so an rc respin of the same version is a distinct file
# (0.20.1-2 vs 0.20.1-3) and the name matches the deb/rpm/tar naming.
# Captured once (`:=`), so prep-mac's guard below checks the very token that
# lands in the file name, and os_version.sh is not re-run per expansion.
MAC_PLATFORM := $(shell $(PKG_DIR)/../.github/bin/os_version.sh)
MAC_PKG = $(TARGET_DIR)/aerospike-asconfig-$(RELEASE)-$(MAC_PLATFORM)-$(ARCH).pkg
DESCRIPTION = "Asconfig, a tool for managing Aerospike database configuration files."
LICENSE = "Apache License 2.0"
URL = "https://github.com/aerospike/asconfig"
VENDOR = "Aerospike, Inc."
ARCH = $(shell uname -m)


.PHONY: all
all: deb rpm tar

.PHONY: deb
deb: prep
	fpm --force \
		--config-files $(CONFIG_DIR) \
		--input-type dir \
		--output-type deb \
		--chdir $(BUILD_DIR)/ \
		--name $(NAME) \
		--version $(PKG_VERSION) \
		--iteration $(DEB_ITERATION) \
		--maintainer $(MAINTAINER) \
		--description $(DESCRIPTION) \
		--license $(LICENSE) \
		--url $(URL) \
		--vendor $(VENDOR) \
		--package $(TARGET_DIR)/$(NAME)_$(PKG_VERSION)-$(DEB_ITERATION)_$(ARCH).deb

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
		--package $(TARGET_DIR)/$(NAME)-$(RPM_VERSION)-$(ITERATION).$(BUILD_DISTRO).$(ARCH).rpm

.PHONY: tar
tar: prep
	fpm --force \
		--config-files $(CONFIG_DIR) \
		--input-type dir \
		--output-type tar \
		--chdir $(BUILD_DIR)/ \
		--name $(NAME) \
		--version $(PKG_VERSION) \
		--maintainer $(MAINTAINER) \
		--description $(DESCRIPTION) \
		--license $(LICENSE) \
		--url $(URL) \
		--vendor $(VENDOR) \
		--package $(TARGET_DIR)/$(NAME)-$(RELEASE)-$(BUILD_DISTRO)-$(ARCH).tar

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
	@test -n "$(MAC_PLATFORM)" || { echo "ERROR: empty macOS platform token -- .github/bin/os_version.sh failed; refusing to build an unnamed pkg" >&2; exit 1; }
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
		--scripts $(PKG_DIR)/mac-scripts \
		$(MAC_PKG)

.PHONY: clean
clean:
	rm -rf $(TARGET_DIR)
	rm -rf $(BUILD_DIR)/opt
	rm -rf $(BUILD_DIR)/var
	rm -rf $(MAC_ROOT)