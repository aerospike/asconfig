package conf

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type ConfigMarshaller struct {
	Format Format
	ConfHandler
}

func NewConfigMarshaller(conf ConfHandler, format Format) ConfigMarshaller {
	return ConfigMarshaller{
		Format:      format,
		ConfHandler: conf,
	}
}

func (cm ConfigMarshaller) MarshalText() (text []byte, err error) {

	switch cm.Format {
	case AsConfig:
		text = []byte(cm.ToConfFile())
	case YAML:
		m := cm.ToMap()
		text, err = yaml.Marshal(m)
	default:
		err = fmt.Errorf("%w %s", ErrInvalidFormat, cm.Format)
	}

	return
}
