package conf

import (
	"errors"
	"fmt"
	"sort"
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

// Validate validates the parsed configuration in ac against
// the Aerospike schema matching ac.aerospikeVersion.
// ValidationErrors is not nil if any errors during validation occur.
// ValidationErrors Error() method outputs a human readable string of validation error details.
// error is not nil if validation, or any other type of error occurs.
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
