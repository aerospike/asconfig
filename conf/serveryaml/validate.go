package serveryaml

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// ValidationError describes a single schema validation failure in a form that
// is independent of the underlying validation library.
type ValidationError struct {
	Context     string
	Field       string
	ErrType     string
	Description string
	Value       interface{}
}

// Error renders a ValidationError in the same shape as
// aerospike-management-lib's asconfig.ValidationErr so that downstream
// formatting can treat them interchangeably.
func (v ValidationError) Error() string {
	return fmt.Sprintf("description: %s, error-type: %s", v.Description, v.ErrType)
}

// Validate validates yamlBytes against the server-native (experimental) JSON
// schema for the given aerospike-server-version. A nil error and a nil/empty
// slice indicates the document is valid against the schema.
func Validate(yamlBytes []byte, version string) ([]ValidationError, error) {
	schemaJSON, err := LoadSchema(version)
	if err != nil {
		return nil, err
	}

	return validateAgainst(yamlBytes, schemaJSON)
}

func validateAgainst(yamlBytes []byte, schemaJSON string) ([]ValidationError, error) {
	docJSON, err := yamlToJSON(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert yaml document to json for validation: %w", err)
	}

	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	docLoader := gojsonschema.NewBytesLoader(docJSON)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("experimental schema validation failed: %w", err)
	}

	if result.Valid() {
		return nil, nil
	}

	errs := make([]ValidationError, 0, len(result.Errors()))
	for _, desc := range result.Errors() {
		errs = append(errs, ValidationError{
			Context:     desc.Context().String(),
			Field:       desc.Field(),
			ErrType:     desc.Type(),
			Description: desc.Description(),
			Value:       desc.Value(),
		})
	}

	return errs, nil
}

// yamlToJSON converts a YAML document to JSON bytes suitable for json-schema
// validation. It uses yaml.v3's strict map[string]any unmarshal so duplicate
// keys fail loudly.
func yamlToJSON(yamlBytes []byte) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return nil, err
	}

	return json.Marshal(doc)
}
