package schema

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
)

// this file is copied from the aerospike kubernetes operator

//go:embed schemas/json/aerospike
var schemas embed.FS

//go:embed schemas/json/aerospike-server
var experimentalSchemas embed.FS

const jsonExtension = ".json"

//nolint:revive // SchemaMap is kept for API compatibility
type SchemaMap map[string]string

func NewSchemaMap() (SchemaMap, error) {
	return collectSchemas(schemas)
}

// NewExperimentalSchemaMap returns the server-native (experimental) YAML
// schemas keyed by version string. These schemas describe the 8.1.0+ native
// YAML format used by asd --experimental.
func NewExperimentalSchemaMap() (SchemaMap, error) {
	return collectSchemas(experimentalSchemas)
}

func collectSchemas(fsys embed.FS) (SchemaMap, error) {
	schema := make(SchemaMap)

	if err := fs.WalkDir(
		fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				content, errRead := fs.ReadFile(fsys, path)
				if errRead != nil {
					return errRead
				}

				// Only include JSON files
				// and extract the key from the filename
				// by removing the directory prefix and the .json extension
				if strings.HasSuffix(path, jsonExtension) {
					baseName := filepath.Base(path)
					key := strings.TrimSuffix(baseName, filepath.Ext(baseName))
					schema[key] = string(content)
				}
			}

			return nil
		},
	); err != nil {
		return nil, err
	}

	return schema, nil
}
