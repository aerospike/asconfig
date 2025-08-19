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

const JSON_EXT = ".json"

type SchemaMap map[string]string

func NewSchemaMap() (SchemaMap, error) {
	schema := make(SchemaMap)

	if err := fs.WalkDir(
		schemas, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				content, err := fs.ReadFile(schemas, path)
				if err != nil {
					return err
				}

				// Only include JSON files
				// and extract the key from the filename
				// by removing the directory prefix and the .json extension
				if strings.HasSuffix(path, JSON_EXT) {
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
