package asconf

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	Invalid  Format = ""
	YAML     Format = "yaml"
	AsConfig Format = "asconfig"
)

var (
	ErrInvalidFormat    = fmt.Errorf("invalid config format")
	ErrConfigValidation = fmt.Errorf("error while validating config")
)

// TODO maybe use mockery here
type confMarshalValidator interface {
	IsValid(log logr.Logger, version string) (bool, []*asconfig.ValidationErr, error)
	ToMap() *asconfig.Conf
	ToConfFile() asconfig.DotConf
	GetFlatMap() *asconfig.Conf
}

type asconf struct {
	cfg                 confMarshalValidator
	logger              *logrus.Logger
	managementLibLogger logr.Logger
	srcFmt              Format
	// TODO decouple output format from asconf, probably pass it as an arg to marshal text
	outFmt           Format
	src              []byte
	aerospikeVersion string
}

func NewAsconf(source []byte, srcFmt, outFmt Format, aerospikeVersion string, logger *logrus.Logger, managementLibLogger logr.Logger) (*asconf, error) {

	ac := &asconf{
		logger:              logger,
		managementLibLogger: managementLibLogger,
		srcFmt:              srcFmt,
		outFmt:              outFmt,
		src:                 source,
		aerospikeVersion:    aerospikeVersion,
	}

	// sets ac.cfg
	err := ac.load()

	return ac, err
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

// Validate validates the parsed configuration in ac against
// the Aerospike schema matching ac.aerospikeVersion.
// ValidationErrors is not nil if any errors during validation occur.
// ValidationErrors Error() method outputs a human readable string of validation error details.
// error is not nil if validation, or any other type of error occurs.
func (ac *asconf) Validate() (*ValidationErrors, error) {

	valid, tempVerrs, err := ac.cfg.IsValid(ac.managementLibLogger, ac.aerospikeVersion)

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

func (ac *asconf) MarshalText() (text []byte, err error) {

	switch ac.outFmt {
	case AsConfig:
		text = []byte(ac.cfg.ToConfFile())
	case YAML:
		m := ac.cfg.ToMap()
		text, err = yaml.Marshal(m)
	default:
		err = fmt.Errorf("%w %s", ErrInvalidFormat, ac.outFmt)
	}

	return
}

func (ac *asconf) GetIntermediateConfig() map[string]any {
	return *ac.cfg.GetFlatMap()
}

func (ac *asconf) load() (err error) {

	switch ac.srcFmt {
	case YAML:
		err = ac.loadYAML()
	case AsConfig:
		err = ac.loadAsConf()
	default:
		return fmt.Errorf("%w %s", ErrInvalidFormat, ac.srcFmt)
	}

	if err != nil {
		return err
	}

	// recreate the management lib config
	// with a sorted config map so that output
	// is always in the same order
	cmap := *ac.cfg.ToMap()

	mutateMap(cmap, []mapping{
		sortLists,
	})

	ac.cfg, err = asconfig.NewMapAsConfig(
		ac.managementLibLogger,
		ac.aerospikeVersion,
		cmap,
	)

	return
}

func (ac *asconf) loadYAML() error {

	var data map[string]any

	err := yaml.Unmarshal(ac.src, &data)
	if err != nil {
		return err
	}

	c, err := asconfig.NewMapAsConfig(
		ac.managementLibLogger,
		ac.aerospikeVersion,
		data,
	)

	if err != nil {
		return fmt.Errorf("failed to initialize asconfig from yaml: %w", err)
	}

	ac.cfg = c

	return nil
}

func (ac *asconf) loadAsConf() error {

	reader := bytes.NewReader(ac.src)

	c, err := asconfig.FromConfFile(ac.managementLibLogger, ac.aerospikeVersion, reader)
	if err != nil {
		return fmt.Errorf("failed to parse asconfig file: %w", err)
	}

	// the aerospike management lib parses asconfig files into
	// a format that its validation rejects
	// this is because the schema files are meant to
	// validate the aerospike kubernetes operator's asconfig yaml format
	// so we modify the map here to match that format
	cmap := *c.ToMap()

	mutateMap(cmap, []mapping{
		typedContextsToObject,
		toPlural,
	})

	c, err = asconfig.NewMapAsConfig(
		ac.managementLibLogger,
		ac.aerospikeVersion,
		cmap,
	)

	if err != nil {
		return err
	}

	ac.cfg = c

	return nil
}
