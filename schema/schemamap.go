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

type Map map[string]string

func NewMap() (Map, error) {
	schema := make(Map)

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

				baseName := filepath.Base(path)
				key := strings.TrimSuffix(baseName, filepath.Ext(baseName))
				schema[key] = string(content)
			}

			return nil
		},
	); err != nil {
		return nil, err
	}

	return schema, nil
}
