package asconf

import (
	"testing"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

func Test_asconf_Validate(t *testing.T) {
	type fields struct {
		cfg                 *asconfig.AsConfig
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
		// {
		// 	name: "yaml-to-conf",
		// 	fields: fields{
		// 		cfg: nil,
		// 	},
		// 	wantErr: false,
		// },
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
