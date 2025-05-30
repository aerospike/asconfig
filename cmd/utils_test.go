//go:build unit

package cmd

import (
	"reflect"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"

	"github.com/spf13/cobra"
)

var mockCmdNoFmt cobra.Command = cobra.Command{}
var mockCmd cobra.Command = cobra.Command{}

func Test_getConfFileFormat(t *testing.T) {
	mockCmdNoFmt.Flags().StringP("format", "F", "yaml", "")
	mockCmdNoFmt.ParseFlags([]string{})

	mockCmd.Flags().StringP("format", "F", "yaml", "")
	mockCmd.ParseFlags([]string{"--format", "conf"})

	type args struct {
		path string
		cmd  *cobra.Command
	}
	tests := []struct {
		name    string
		args    args
		want    asConf.Format
		wantErr bool
	}{
		{
			name: "p1",
			args: args{
				path: "conf.yaml",
				cmd:  &mockCmdNoFmt,
			},
			want:    asConf.YAML,
			wantErr: false,
		},
		{
			name: "p2",
			args: args{
				path: "conf.conf",
				cmd:  &mockCmdNoFmt,
			},
			want:    asConf.AeroConfig,
			wantErr: false,
		},
		{
			name: "p3",
			args: args{
				path: "conf.yaml",
				cmd:  &mockCmd,
			},
			want:    asConf.AeroConfig,
			wantErr: false,
		},
		{
			name: "n1",
			args: args{
				path: "../testdata/sources/all_flash_cluster_cr.bad",
				cmd:  &mockCmdNoFmt,
			},
			want:    asConf.Invalid,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConfFileFormat(tt.args.path, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("getConfFileFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getConfFileFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
