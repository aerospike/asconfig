package conf

import "fmt"

type Format string

const (
	Invalid  Format = ""
	YAML     Format = "yaml"
	AsConfig Format = "asconfig"
)

var ErrInvalidFormat = fmt.Errorf("invalid config format")
