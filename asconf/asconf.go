package asconf

import (
	"bytes"
	"fmt"

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
	JSON     Format = "json"
)

var (
	errInvalidFormat    = fmt.Errorf("invalid config format")
	errConfigValidation = fmt.Errorf("error while validating config")
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
	outFmt              Format
	src                 []byte
	aerospikeVersion    string
}

func NewAsconf(source []byte, srcFmt, outFmt Format, aerospikeVersion string, logger *logrus.Logger, managementLibLogger logr.Logger) (ac *asconf, err error) {

	ac = &asconf{
		logger:              logger,
		managementLibLogger: managementLibLogger,
		srcFmt:              srcFmt,
		outFmt:              outFmt,
		src:                 source,
		aerospikeVersion:    aerospikeVersion,
	}

	// sets AsConfig
	err = ac.load()
	if err != nil {
		return
	}

	return
}

func (ac *asconf) Validate() error {

	valid, validationErrors, err := ac.cfg.IsValid(ac.managementLibLogger, ac.aerospikeVersion)

	if len(validationErrors) > 0 {
		for _, e := range validationErrors {
			ac.logger.Errorf("Aerospike config validation error: %+v", e)
		}
	}

	if !valid || err != nil || len(validationErrors) > 0 {
		return fmt.Errorf("%w, %w", errConfigValidation, err)
	}

	return err
}

func (ac *asconf) MarshalText() (text []byte, err error) {

	switch ac.outFmt {
	case AsConfig:
		text = []byte(ac.cfg.ToConfFile())
	case YAML:
		m := ac.cfg.ToMap()
		text, err = yaml.Marshal(m)
	default:
		err = fmt.Errorf("%w %s", errInvalidFormat, ac.outFmt)
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
		return fmt.Errorf("%w %s", errInvalidFormat, ac.srcFmt)
	}

	if err != nil {
		return err
	}

	cmap := *ac.cfg.ToMap()

	mapToStats(cmap, []mapping{
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

	mapToStats(cmap, []mapping{
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
