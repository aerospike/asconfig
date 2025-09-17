package conf

import (
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
)

//nolint:revive // ConfHandler is kept for API compatibility
type ConfHandler interface {
	IsValid(log logr.Logger, version string) (bool, []*asconfig.ValidationErr, error)
	ToMap() *asconfig.Conf
	ToConfFile() asconfig.DotConf
	GetFlatMap() *asconfig.Conf
}
