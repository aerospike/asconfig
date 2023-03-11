package cmd

import (
	"fmt"
	"reflect"
)

// TODO move this to its own package

const (
	cmdNameKey  = "commandName"
	flagNameKey = "flagName"
	argNameKey  = "argumentName"
	valueKey    = "value"
	fileKey     = "file"
	countKey    = "count"
	expectedKey = "expected"
)

const (
	Fatal int = iota
	Error
	Warning
	Info
	Verbose
	Debug
)

// structToKeysAndValues converts a struct to a list of key value pairs of the format
// expected by logr. Nested structs, maps, etc, are not flattened.
// all struct fields must be exported
func structToKeysAndValues(v any) ([]any, error) {
	var res []any

	vval := reflect.ValueOf(v)
	vkind := vval.Kind()
	vtype := vval.Type()

	if vkind != reflect.Struct {
		return res, fmt.Errorf("structToKeysAndValues got unsupported type: %s", vkind.String())
	}

	fieldCount := vval.NumField()
	res = make([]any, fieldCount*2)

	for i := 0; i < fieldCount; i++ {
		j := i * 2
		res[j] = vtype.Field(i).Name
		res[j+1] = vval.Field(i).Interface()
	}

	return res, nil
}
