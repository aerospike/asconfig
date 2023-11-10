package metadata

import (
	"fmt"
	"regexp"
)

var commentChar string
var findComments *regexp.Regexp

func init() {
	commentChar = "#"
	findComments = regexp.MustCompile(commentChar + `(?m)\s*(.+):\s*(.+)\s*$`)
}

// type Data struct {
// 	AerospikeVersion string `aero-meta:"aerospike-server-version"`
// 	AsadmVersion     string `aero-meta:"asadm-version"`
// 	AsconfigVersion  string `aero-meta:"asconfig-version"`
// }

func UnmarshalText(src []byte, dst map[string]string) error {
	matches := findComments.FindAllSubmatch(src, -1)

	for _, match := range matches {
		// 0 index is entire line
		k := match[1]
		v := match[2]
		// only save the first occurrence of k
		if _, ok := dst[string(k)]; !ok {
			dst[string(k)] = string(v)
		}
	}

	return nil
}

func formatLine(k string, v any) string {
	fmtStr := "%s %s: %v"
	return fmt.Sprintf(fmtStr, commentChar, k, v)
}

func MarshalText(src map[string]string) ([]byte, error) {
	res := []byte{}

	for k, v := range src {
		res = append(res, []byte(formatLine(k, v)+"\n")...)
	}

	return res, nil
}
