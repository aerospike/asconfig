//go:build unit

package conf

import (
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-management-lib/asconfig"
)

func Test_asconf_MarshalText(t *testing.T) {
	type fields struct {
		cfg    ConfHandler
		outFmt Format
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
			ac := NewConfigMarshaller(tt.fields.cfg, tt.fields.outFmt)
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
