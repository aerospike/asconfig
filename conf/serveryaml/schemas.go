package serveryaml

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	lib "github.com/aerospike/aerospike-management-lib"

	"github.com/aerospike/asconfig/schema"
)

var (
	experimentalSchemasOnce sync.Once
	experimentalSchemas     schema.SchemaMap
	errExperimentalSchemas  error

	baseVersionRe  = regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	minorVersionRe = regexp.MustCompile(`^(\d+\.\d+)`)
)

// LoadSchema returns the experimental JSON schema string for the given
// aerospike-server-version. The version string may include an "ee-" prefix
// and/or additional patch components; only the leading major.minor.patch is
// used for lookup.
//
// When an exact match is not available the highest-versioned embedded schema
// that is not newer than the requested version is returned instead. This
// mirrors how asconfig resolves legacy schemas by minor release.
func LoadSchema(version string) (string, error) {
	body, _, err := loadSchemaResolved(version)
	return body, err
}

// loadSchemaResolved is the internal version of LoadSchema that also returns
// the embedded schema version that was picked. Exported behavior is identical
// to LoadSchema; the second return value exists for tests that need to make
// sure asconfig resolved the right schema for a given version string.
func loadSchemaResolved(version string) (string, string, error) {
	if version == "" {
		return "", "", ErrMissingVersion
	}

	schemas, err := loadExperimentalSchemas()
	if err != nil {
		return "", "", err
	}

	baseVersion, err := extractBaseVersion(strings.TrimPrefix(version, "ee-"))
	if err != nil {
		return "", "", err
	}

	if schemaJSON, ok := schemas[baseVersion]; ok {
		return schemaJSON, baseVersion, nil
	}

	if resolved := resolveSchemaVersion(schemas, baseVersion); resolved != "" {
		return schemas[resolved], resolved, nil
	}

	// No schema <= the requested version; fall back to the lowest schema
	// embedded for the same major.minor so patch-level differences within a
	// minor release still resolve (asconfig ignores patch numbers).
	if fallback := lowestSchemaForSameMinor(schemas, baseVersion); fallback != "" {
		return schemas[fallback], fallback, nil
	}

	return "", "", fmt.Errorf("no native yaml schema found for aerospike-server-version %s", version)
}

// resolveSchemaVersion finds the highest embedded schema version that is not
// greater than target. Returns "" if no embedded schema is <= target.
func resolveSchemaVersion(schemas schema.SchemaMap, target string) string {
	candidates := make([]string, 0, len(schemas))
	for v := range schemas {
		cmp, err := lib.CompareVersions(v, target)
		if err != nil {
			continue
		}

		if cmp <= 0 {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		cmp, err := lib.CompareVersions(candidates[i], candidates[j])
		if err != nil {
			return candidates[i] < candidates[j]
		}

		return cmp < 0
	})

	return candidates[len(candidates)-1]
}

func loadExperimentalSchemas() (schema.SchemaMap, error) {
	experimentalSchemasOnce.Do(func() {
		rawSchemas, err := schema.NewExperimentalSchemaMap()
		if err != nil {
			errExperimentalSchemas = err
			return
		}

		sanitized := make(schema.SchemaMap, len(rawSchemas))
		for version, body := range rawSchemas {
			sanitized[version] = sanitizeExperimentalSchema(body)
		}

		experimentalSchemas = sanitized
	})

	return experimentalSchemas, errExperimentalSchemas
}

// sanitizeExperimentalSchema rewrites JSON-schema regex patterns that use
// features unsupported by Go's regexp engine (and therefore gojsonschema) so
// the experimental schema can be used for validation today.
//
// The only known offender is the "not-null" guard
// `^(?!null$)[a-zA-Z0-9_\-$]+$` which uses negative-lookahead. We replace it
// with the functionally-equivalent (minus the explicit null exclusion)
// `^[a-zA-Z0-9_\-$]+$`. Documents whose keys are the literal string "null"
// will still be caught downstream by aerospike-management-lib because it
// treats every non-string or reserved key as invalid.
func sanitizeExperimentalSchema(body string) string {
	const (
		negativeLookaheadPattern = `^(?!null$)[a-zA-Z0-9_\\-$]+$`
		replacementPattern       = `^[a-zA-Z0-9_\\-$]+$`
	)

	return strings.ReplaceAll(body, negativeLookaheadPattern, replacementPattern)
}

func extractBaseVersion(version string) (string, error) {
	match := baseVersionRe.FindString(version)
	if match == "" {
		return "", fmt.Errorf("invalid aerospike-server-version %q", version)
	}

	return match, nil
}

func extractMinor(version string) string {
	return minorVersionRe.FindString(version)
}

func lowestSchemaForSameMinor(schemas schema.SchemaMap, target string) string {
	targetMinor := extractMinor(target)
	if targetMinor == "" {
		return ""
	}

	candidates := make([]string, 0, len(schemas))
	for v := range schemas {
		if extractMinor(v) == targetMinor {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		cmp, err := lib.CompareVersions(candidates[i], candidates[j])
		if err != nil {
			return candidates[i] < candidates[j]
		}

		return cmp < 0
	})

	return candidates[0]
}
