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

func Test_jsonToConfigContext(t *testing.T) {
	type args struct {
		jsonConfig any
		context    string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "positive",
			args: args{
				jsonConfig: map[string]any{
					"test0": map[string]any{
						"name":  "nested0",
						"test1": "test2",
					},
					"test3": "test4",
					"test5": []any{
						"test6",
						map[string]any{
							"name":  "nested1",
							"test7": "test8",
							"test9": "test10",
							"test11": []any{
								"test12",
								"test13",
								map[string]any{
									"name":   "nested2",
									"test14": "test15",
								},
							},
						},
					},
				},
				context: "test5.1.test11.2",
			},
			want: "test5.nested1.test11.nested2",
		},
		{
			name: "negative context is not a slice",
			args: args{
				jsonConfig: map[string]any{
					"test0": map[string]any{
						"name":  "nested0",
						"test1": "test2",
					},
					"test3": "test4",
				},
				context: "test0.1.test1",
			},
			wantErr: true,
		},
		{
			name: "negative context is not a map",
			args: args{
				jsonConfig: map[string]any{
					"test0": []any{
						"test1",
					},
				},
				context: "test0.0",
			},
			wantErr: true,
		},
		{
			name: "negative name not found",
			args: args{
				jsonConfig: map[string]any{
					"test0": []any{
						map[string]any{
							"test1": "test2",
						},
					},
				},
				context: "test0.0",
			},
			wantErr: true,
		},
		{
			name: "negative name is not a string",
			args: args{
				jsonConfig: map[string]any{
					"test0": []any{
						map[string]any{
							"name":  1,
							"test1": "test2",
						},
					},
				},
				context: "test0.0",
			},
			wantErr: true,
		},
		{
			name: "negative context is not a map, non numeric key",
			args: args{
				jsonConfig: map[string]any{
					"test0": []any{
						"test1",
					},
				},
				context: "test0.test1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonToConfigContext(tt.args.jsonConfig, tt.args.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("jsonToConfigContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("jsonToConfigContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
