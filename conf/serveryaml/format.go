// Package serveryaml provides translation, schema validation, and version
// gating for the Aerospike server-native (experimental) YAML format.
//
// The package is self-contained with no dependencies on asconfig/cmd or cobra
// so that it can be lifted into aerospike-management-lib later with minimal
// churn.
package serveryaml

import (
	"errors"
	"fmt"
	"strings"

	lib "github.com/aerospike/aerospike-management-lib"
)

// MinSupportedVersion is the lowest aerospike-server-version for which the
// server-native YAML format is recognized by asconfig. asconfig does not
// consider the patch component, so any 8.1.x release qualifies.
const MinSupportedVersion = "8.1.0"

// ErrUnsupportedVersion is returned when IsSupportedVersion reports false.
var ErrUnsupportedVersion = fmt.Errorf(
	"server-native yaml requires aerospike-server-version >= %s",
	MinSupportedVersion,
)

// ErrMissingVersion is returned when callers require a version for gating but
// none was supplied.
var ErrMissingVersion = errors.New("server-native yaml requires an aerospike-server-version")

// IsSupportedVersion reports whether the given aerospike-server-version is new
// enough to use the server-native YAML format. Enterprise ("ee-") prefixes are
// stripped before comparison to match the rest of the codebase.
func IsSupportedVersion(version string) (bool, error) {
	version = strings.TrimPrefix(version, "ee-")

	if version == "" {
		return false, nil
	}

	cmp, err := lib.CompareVersions(version, MinSupportedVersion)
	if err != nil {
		return false, err
	}

	return cmp >= 0, nil
}
