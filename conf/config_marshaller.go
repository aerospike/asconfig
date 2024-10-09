package conf

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type ConfigMarshaller struct {
	Format Format
	Handler
}

func NewConfigMarshaller(conf Handler, format Format) ConfigMarshaller {
	return ConfigMarshaller{
		Format:  format,
		Handler: conf,
	}
}

func (cm ConfigMarshaller) MarshalText() (text []byte, err error) {
	switch cm.Format {
	case AsConfig:
		text = []byte(cm.ToConfFile())
	case YAML:
		m := cm.ToMap()
		text, err = yaml.Marshal(m)
	case Invalid:
		err = fmt.Errorf("%w %s", ErrInvalidFormat, cm.Format)
	default:
		err = fmt.Errorf("%w %s", ErrInvalidFormat, cm.Format)
	}

	return
}
