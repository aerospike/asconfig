package conf

import (
	"fmt"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"gopkg.in/yaml.v3"
)

type ConfigMarshaller struct {
	Format asConf.Format
	ConfHandler
}

func NewConfigMarshaller(conf ConfHandler, format asConf.Format) ConfigMarshaller {
	return ConfigMarshaller{
		Format:      format,
		ConfHandler: conf,
	}
}

func (cm ConfigMarshaller) MarshalText() (text []byte, err error) {
	switch cm.Format {
	case asConf.AeroConfig:
		text = []byte(cm.ToConfFile())
	case asConf.YAML:
		m := cm.ToMap()
		text, err = yaml.Marshal(m)
	case asConf.Invalid:
		err = fmt.Errorf("%w %s", asConf.ErrInvalidFormat, cm.Format)
	default:
		err = fmt.Errorf("%w %s", asConf.ErrInvalidFormat, cm.Format)
	}

	return
}
