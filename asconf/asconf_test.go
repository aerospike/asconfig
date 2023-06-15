//go:build unit
// +build unit

package asconf

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

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

var _testYaml = `
namespaces:
    - index-type:
        mounts:
            - /test/dev/xvdf-index
        mounts-size-limit: 4294967296
        type: flash
      memory-size: 3000000000
      name: test
      replication-factor: 2
      storage-engine:
        devices:
            - /test/dev/xvdf
        type: device
`

// {
// 	name: "yaml-to-conf",
// 	fields: fields{
// 		cfg: &mockCFG{
// 			valid:            true,
// 			err:              nil,
// 			validationErrors: []*asconfig.ValidationErr{},
// 			confMap: &asconfig.Conf{
// 				"namespaces": []string{"ns1", "ns1"},
// 			},
// 			confText: "namespace ns1 {}\n namespace ns2 {}",
// 			flatConf: &asconfig.Conf{
// 				"namespaces.ns1": "device",
// 				"namespaces.ns2": "memory",
// 			},
// 		},
// 		logger:              logrus.New(),
// 		managementLibLogger: logrusr.New(logrus.New()),
// 		srcFmt:              YAML,
// 		outFmt:              AsConfig,
// 		src:                 []byte(_testYaml),
// 		aerospikeVersion:    "6.2.0.2",
// 	},
// 	wantErr: false,
// },

func Test_asconf_Validate(t *testing.T) {
	type fields struct {
		cfg                 confMarshalValidator
		logger              *logrus.Logger
		managementLibLogger logr.Logger
		srcFmt              Format
		outFmt              Format
		src                 []byte
		aerospikeVersion    string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "pos1",
			fields: fields{
				cfg: &mockCFG{
					valid:            true,
					err:              nil,
					validationErrors: []*asconfig.ValidationErr{},
				},
				logger: logrus.New(),
			},
			wantErr: false,
		},
		{
			name: "neg1",
			fields: fields{
				cfg: &mockCFG{
					valid:            false,
					err:              nil,
					validationErrors: []*asconfig.ValidationErr{},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "neg2",
			fields: fields{
				cfg: &mockCFG{
					valid:            true,
					err:              fmt.Errorf("test_err"),
					validationErrors: []*asconfig.ValidationErr{},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "neg3",
			fields: fields{
				cfg: &mockCFG{
					valid:            true,
					err:              nil,
					validationErrors: []*asconfig.ValidationErr{{}},
				},
				logger: logrus.New(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &asconf{
				cfg:                 tt.fields.cfg,
				logger:              tt.fields.logger,
				managementLibLogger: tt.fields.managementLibLogger,
				srcFmt:              tt.fields.srcFmt,
				outFmt:              tt.fields.outFmt,
				src:                 tt.fields.src,
				aerospikeVersion:    tt.fields.aerospikeVersion,
			}
			if err := ac.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("asconf.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_asconf_MarshalText(t *testing.T) {
	type fields struct {
		cfg                 confMarshalValidator
		logger              *logrus.Logger
		managementLibLogger logr.Logger
		srcFmt              Format
		outFmt              Format
		src                 []byte
		aerospikeVersion    string
	}
	tests := []struct {
		name     string
		fields   fields
		wantText []byte
		wantErr  bool
	}{
		{
			name: "valid asconfig format",
			fields: fields{
				cfg: &mockCFG{
					confText: "namespace ns1 {}\n namespace ns2 {}",
				},
				outFmt: AsConfig,
			},
			wantErr:  false,
			wantText: []byte("namespace ns1 {}\n namespace ns2 {}"),
		},
		{
			name: "valid yaml format",
			fields: fields{
				cfg: &mockCFG{
					confMap: &asconfig.Conf{
						"namespaces": "ns1",
					},
					confText: "",
				},
				outFmt: YAML,
			},
			wantErr:  false,
			wantText: []byte("namespaces: ns1\n"),
		},
		{
			name: "invalid format",
			fields: fields{
				outFmt: Invalid,
			},
			wantErr:  true,
			wantText: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &asconf{
				cfg:                 tt.fields.cfg,
				logger:              tt.fields.logger,
				managementLibLogger: tt.fields.managementLibLogger,
				srcFmt:              tt.fields.srcFmt,
				outFmt:              tt.fields.outFmt,
				src:                 tt.fields.src,
				aerospikeVersion:    tt.fields.aerospikeVersion,
			}
			gotText, err := ac.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("asconf.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotText, tt.wantText) {
				t.Errorf("asconf.MarshalText() = %v, want %v", gotText, tt.wantText)
			}
		})
	}
}

func Test_asconf_GetIntermediateConfig(t *testing.T) {
	type fields struct {
		cfg                 confMarshalValidator
		logger              *logrus.Logger
		managementLibLogger logr.Logger
		srcFmt              Format
		outFmt              Format
		src                 []byte
		aerospikeVersion    string
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]any
	}{
		{
			name: "return flat conf",
			fields: fields{
				cfg: &mockCFG{
					flatConf: &configMap{"ns": 1},
				},
			},
			want: map[string]any{"ns": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &asconf{
				cfg:                 tt.fields.cfg,
				logger:              tt.fields.logger,
				managementLibLogger: tt.fields.managementLibLogger,
				srcFmt:              tt.fields.srcFmt,
				outFmt:              tt.fields.outFmt,
				src:                 tt.fields.src,
				aerospikeVersion:    tt.fields.aerospikeVersion,
			}
			if got := ac.GetIntermediateConfig(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("asconf.GetIntermediateConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
