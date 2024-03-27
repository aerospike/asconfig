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
	ErrConfigValidation = fmt.Errorf("error while validating config")
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
		verr := ValidationErr{
			ValidationErr: *v,
		}
		verrs.Errors = append(verrs.Errors, verr)
	}

	if !valid || err != nil || len(verrs.Errors) > 0 {

		config := cv.ToMap()

		jsonConfigStr, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}

		jsonConfig := map[string]any{}
		err = json.Unmarshal(jsonConfigStr, &jsonConfig)

		// check the context of each error and use that context to get the name
		// of the field that is causing the error from the json config
		for i, verr := range verrs.Errors {
			context, _ := strings.CutPrefix(verr.Context, "(root).")
			context, err := jsonToConfigContext(jsonConfig, context)
			if err != nil {
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

type ValidationErr struct {
	asconfig.ValidationErr
}

type VErrSlice []ValidationErr

func (a VErrSlice) Len() int           { return len(a) }
func (a VErrSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a VErrSlice) Less(i, j int) bool { return strings.Compare(a[i].Error(), a[j].Error()) == -1 }

// Outputs a human readable string of validation error details.
// error is not nil if validation, or any other type of error occurs.
func (o ValidationErr) Error() string {
	verrTemplate := "description: %s, error-type: %s"
	return fmt.Sprintf(verrTemplate, o.Description, o.ErrType)
}

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

func jsonToConfigContext(jsonConfig any, context string) (string, error) {
	split := strings.SplitN(context, ".", 2)
	key := split[0]

	var res string

	if index, err := strconv.Atoi(key); err == nil {
		jsonSlice, ok := jsonConfig.([]any)
		if !ok {
			return "", fmt.Errorf("context is not a slice in json config at: %s", context)
		}

		indexedMap, ok := jsonSlice[index].(map[string]any)
		if !ok {
			return "", fmt.Errorf("context is not a map in json config at: %s", context)
		}

		name, ok := indexedMap["name"]
		if !ok {
			return "", fmt.Errorf("name not found in json config at: %s", context)
		}

		if nameStr, ok := name.(string); !ok {
			return "", fmt.Errorf("name is not a string in json config at: %v", context)
		} else {
			res = nameStr
		}

		jsonConfig = indexedMap
	} else {
		jsonMap, ok := jsonConfig.(map[string]any)
		if !ok {
			return "", fmt.Errorf("context is not a map in json config at: %s", context)
		}

		jsonConfig, ok = jsonMap[key]
		if !ok {
			return "", fmt.Errorf("context not found in json config at: %s", key)
		}

		res = key
	}

	if len(split) > 1 {
		val, err := jsonToConfigContext(jsonConfig, split[1])
		if err != nil {
			return "", err
		}

		res = fmt.Sprintf("%s.%s", res, val)
	}

	return res, nil
}
