package metadata_test

import (
	"reflect"
	"testing"

	"github.com/aerospike/asconfig/conf/metadata"
)

var testBasic = `
# comment about metadata
# a: b
other data
`

var testConf = `
# comment about metadata
# aerospike-server-version: 6.4.0.1
# asconfig-version: 0.12.0
# asadm-version:	2.20.0

#

logging {
	
	file /dummy/file/path2 {
		context any info # aerospike-server-version: collide
	}
}
`

var testConfNoMeta = `
namespace ns2 {
	replication-factor 2
	memory-size 8G
	index-type shmem  # comment mid config
	sindex-type shmem
	storage-engine memory
}
# comment
`

var testConfPartialMeta = `
namespace ns1 {
	replication-factor 2
	memory-size 4G

	index-type flash {
        mount /dummy/mount/point1 /test/mount2
        mounts-high-water-pct 30
        mounts-size-limit 10G
    }

	# comment about metadata
	# aerospike-server-version: 6.4.0.1
	# other-item: a long value
`

func TestUnmarshal(t *testing.T) {
	type args struct {
		src []byte
		dst map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "t1",
			args: args{
				src: []byte(testConf),
				dst: map[string]string{},
			},
			want: map[string]string{
				"aerospike-server-version": "6.4.0.1",
				"asadm-version":            "2.20.0",
				"asconfig-version":         "0.12.0",
			},
			wantErr: false,
		},
		{
			name: "t2",
			args: args{
				src: []byte(testConfNoMeta),
				dst: map[string]string{},
			},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "t3",
			args: args{
				src: []byte(testConfPartialMeta),
				dst: map[string]string{},
			},
			want: map[string]string{
				"aerospike-server-version": "6.4.0.1",
				"other-item":               "a long value",
			},
			wantErr: false,
		},
		{
			name: "t4",
			args: args{
				src: []byte(testBasic),
				dst: map[string]string{},
			},
			want: map[string]string{
				"a": "b",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := metadata.Unmarshal(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("Unmarshal() = %v, want %v", tt.args.dst, tt.want)
			}
		})
	}
}

var testMarshalMetaComplete = `# aerospike-server-version: 7.0.0.0
# asadm-version: 2.20.0
# asconfig-version: 0.12.0
`

var testMarshalMetaNone = ""

var testMarshalMetaPartial = `# aerospike-server-version: 6.4.0
`

func TestMarshalText(t *testing.T) {
	type args struct {
		src map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "t1",
			args: args{
				src: map[string]string{
					"aerospike-server-version": "7.0.0.0",
					"asadm-version":            "2.20.0",
					"asconfig-version":         "0.12.0",
				},
			},
			want:    []byte(testMarshalMetaComplete),
			wantErr: false,
		},
		{
			name: "t2",
			args: args{
				src: map[string]string{},
			},
			want:    []byte(testMarshalMetaNone),
			wantErr: false,
		},
		{
			name: "t3",
			args: args{
				src: map[string]string{
					"aerospike-server-version": "6.4.0",
				},
			},
			want: []byte(testMarshalMetaPartial),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := metadata.Marshal(tt.args.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Marshal() = %v, want %v", got, tt.want)
			}
		})
	}
}
