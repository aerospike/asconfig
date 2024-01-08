//go:build unit

package conf

import (
	"fmt"
	"testing"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

func Test_asconf_Validate(t *testing.T) {
	type fields struct {
		cfg                 ConfHandler
		logger              *logrus.Logger
		managementLibLogger logr.Logger
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
			ac := NewConfigValidator(tt.fields.cfg, tt.fields.managementLibLogger, tt.fields.aerospikeVersion)
			if _, err := ac.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("asconf.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
