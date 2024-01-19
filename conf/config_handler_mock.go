//go:build unit

package conf

import (
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
)

// TODO: Use a gomock instead. It is more robust in its assertions.

type mockCFG struct {
	valid            bool
	err              error
	validationErrors []*asconfig.ValidationErr
	confMap          *asconfig.Conf
	confText         string
	flatConf         *asconfig.Conf
}

func (o *mockCFG) IsValid(log logr.Logger, version string) (bool, []*asconfig.ValidationErr, error) {
	return o.valid, o.validationErrors, o.err
}

func (o *mockCFG) ToMap() *asconfig.Conf {
	return o.confMap
}

func (o *mockCFG) ToConfFile() asconfig.DotConf {
	return o.confText
}

func (o *mockCFG) GetFlatMap() *asconfig.Conf {
	return o.flatConf
}
