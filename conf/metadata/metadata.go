package metadata

import (
	"fmt"
	"regexp"
	"sort"
)

var commentChar string
var findComments *regexp.Regexp

func init() {
	commentChar = "#"
	// findComments matches text of the form `<commentChar> <key>: <val>`
	// for example, parsing...
	// # comment about metadata
	// # a: b
	// other data
	// matches
	// # a: b
	findComments = regexp.MustCompile(commentChar + `(?m)\s*(.+):\s*(.+)\s*$`)
}

func Unmarshal(src []byte, dst map[string]string) error {
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

func Marshal(src map[string]string) ([]byte, error) {
	res := []byte{}
	lines := make([]string, len(src))

	for k, v := range src {
		lines = append(lines, formatLine(k, v)+"\n")
	}

	sort.Strings(lines)

	for _, v := range lines {
		res = append(res, []byte(v)...)
	}

	return res, nil
}
