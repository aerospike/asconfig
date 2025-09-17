package conf

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
)

var (
	ErrConfigValidation = errors.New("error while validating config")
	ErrContextNotSlice  = errors.New("context is not a slice in json config")
	ErrIndexOutOfBounds = errors.New("index out of bounds json config")
	ErrContextNotMap    = errors.New("context is not a map in json config")
	ErrNameNotFound     = errors.New("name not found in json config")
	ErrNameNotString    = errors.New("name is not a string in json config")
	ErrContextNotFound  = errors.New("context not found in json config")
)

type ConfigValidator struct {
	ConfHandler

	mgmtLogger logr.Logger
	version    string
}

func NewConfigValidator(confHandler ConfHandler, mgmtLogger logr.Logger, version string) *ConfigValidator {
	return &ConfigValidator{
		ConfHandler: confHandler,
		mgmtLogger:  mgmtLogger,
		version:     version,
	}
}

// Validate validates the parsed configuration against the schema for the given versions.
// ValidationErrors is not nil if any errors occur during validation.
func (cv *ConfigValidator) Validate() (*ValidationErrors, error) {
	valid, tempVerrs, err := cv.IsValid(cv.mgmtLogger, cv.version)

	verrs := ValidationErrors{}

	for _, v := range tempVerrs {
		verr := ValidationError{
			ValidationErr: *v,
		}
		verrs.Errors = append(verrs.Errors, verr)
	}

	if !valid || err != nil || len(verrs.Errors) > 0 {
		config := cv.ToMap()

		jsonConfigStr, errMarshal := json.Marshal(config)
		if errMarshal != nil {
			return nil, errMarshal
		}

		jsonConfig := map[string]any{}
		err = json.Unmarshal(jsonConfigStr, &jsonConfig)

		// check the context of each error and use that context to get the name
		// of the field that is causing the error from the json config
		for i, verr := range verrs.Errors {
			context, _ := strings.CutPrefix(verr.Context, "(root).")

			context, errContext := jsonToConfigContext(jsonConfig, context)
			if errContext != nil {
				// if we can't associate the error with its
				// corresponding field, just use the current context
				continue
			}

			verrs.Errors[i].Context = context
		}

		return &verrs, errors.Join(ErrConfigValidation, err)
	}

	return nil, nil
}

type ValidationError struct {
	asconfig.ValidationErr
}

type VErrSlice []ValidationError

func (a VErrSlice) Len() int           { return len(a) }
func (a VErrSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a VErrSlice) Less(i, j int) bool { return a[i].Error() < a[j].Error() }

// Error outputs a human readable string of validation error details.
// error is not nil if validation, or any other type of error occurs.
func (o *ValidationError) Error() string {
	verrTemplate := "description: %s, error-type: %s"
	return fmt.Sprintf(verrTemplate, o.Description, o.ErrType)
}

//nolint:errname // TODO: fix this
type ValidationErrors struct {
	Errors VErrSlice
}

func (o ValidationErrors) Error() string {
	errorsByContext := map[string]VErrSlice{}

	sort.Sort(o.Errors)

	for _, err := range o.Errors {
		errorsByContext[err.Context] = append(errorsByContext[err.Context], err)
	}

	contexts := []string{}
	for ctx := range errorsByContext {
		contexts = append(contexts, ctx)
	}

	sort.Strings(contexts)

	errString := ""

	for _, ctx := range contexts {
		errString += fmt.Sprintf("context: %s\n", ctx)

		errList := errorsByContext[ctx]
		for _, err := range errList {
			// filter "Must validate one and only one schema " errors
			// I have never seen a useful one and they seem to always be
			// accompanied by another more useful error that will be displayed
			if err.ErrType == "number_one_of" {
				continue
			}

			errString += fmt.Sprintf("\t- %s\n", err.Error())
		}
	}

	return errString
}

// jsonToConfigContext takes a json config and a context string and returns a copy
// of the context string with the field names instead of list indexes.
// For example, if the context is "namespaces.0", and the json config is:
//
//	{
//	  "namespaces": [
//	    {
//	      "name": "test"
//	    }
//	  ]
//	}
//
// The function will return "namespaces.test"
// An error is returned if the json or context are not in the expected management lib format.
func jsonToConfigContext(jsonConfig any, context string) (string, error) {
	const maxSplitParts = 2

	split := strings.SplitN(context, ".", maxSplitParts)
	key := split[0]

	var res string

	// check if key is an index
	if index, err := strconv.Atoi(key); err == nil {
		var errIndexed error

		res, jsonConfig, errIndexed = handleIndexedKey(jsonConfig, index, context)
		if errIndexed != nil {
			return "", errIndexed
		}
	} else {
		// if key is not an index, then context should be a map
		jsonMap, ok := jsonConfig.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%w at: %s", ErrContextNotMap, context)
		}

		// set jsonConfig to the object at key
		jsonConfig, ok = jsonMap[key]
		if !ok {
			return "", fmt.Errorf("%w at: %s", ErrContextNotFound, key)
		}

		// set res to the context key
		res = key
	}

	if len(split) > 1 {
		// if we have more context to parse, recurse
		val, err := jsonToConfigContext(jsonConfig, split[1])
		if err != nil {
			return "", err
		}

		res = fmt.Sprintf("%s.%s", res, val)
	}

	return res, nil
}

// handleIndexedKey processes an indexed key from a JSON slice context.
func handleIndexedKey(jsonConfig any, index int, context string) (string, any, error) {
	// if key is an index, then context should be a slice
	jsonSlice, ok := jsonConfig.([]any)
	if !ok {
		return "", nil, fmt.Errorf("%w at: %s", ErrContextNotSlice, context)
	}

	// check if index is out of bounds
	if len(jsonSlice) <= index {
		return "", nil, fmt.Errorf("%w at: %s", ErrIndexOutOfBounds, context)
	}

	// the indexed object should be a map
	indexedMap, ok := jsonSlice[index].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("%w at: %s", ErrContextNotMap, context)
	}

	// get the name field from the indexed object
	name, ok := indexedMap["name"]
	if !ok {
		return "", nil, fmt.Errorf("%w at: %s", ErrNameNotFound, context)
	}

	nameStr, ok := name.(string)
	if !ok {
		return "", nil, fmt.Errorf("%w at: %v", ErrNameNotString, context)
	}

	return nameStr, indexedMap, nil
}
