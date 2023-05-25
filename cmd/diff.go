package cmd

import (
	"aerospike/asconfig/asconf"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
)

var (
	errConfigsDiffer = fmt.Errorf("configuration files are not equal")
)

func init() {
	rootCmd.AddCommand(diffCommand)
}

var diffCommand = newDiffCommand()

func newDiffCommand() *cobra.Command {
	res := &cobra.Command{
		Use:   "diff [flags] <path/to/config1> <path/to/config2>",
		Short: "Diff yaml and conf Aerospike configuration files.",
		Long: `Diff is used to compare differences between Aerospike configuration files.
				It is used on two files of any type supported by the asconfig tool.
				Schema validation is not performed on either file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running diff command")

			if len(args) < 2 {
				return fmt.Errorf("diff requires 2 file paths as arguments")
			}

			path1 := args[0]
			path2 := args[1]

			ext1 := filepath.Ext(path1)[1:]
			fmt1, err := asconf.ParseFmtString(ext1)
			if err != nil {
				return err
			}

			ext2 := filepath.Ext(path2)[1:]
			fmt2, err := asconf.ParseFmtString(ext2)
			if err != nil {
				return err
			}

			if ext2 != ext1 {
				return fmt.Errorf("mismatched file formats, detected %s and %s", ext1, ext2)
			}

			f1, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			f2, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}

			conf1, err := asconf.NewAsconf(
				f1,
				fmt1,
				asconf.JSON,
				"",
				logger,
				managementLibLogger,
			)
			if err != nil {
				return err
			}

			conf2, err := asconf.NewAsconf(
				f2,
				fmt2,
				asconf.JSON,
				"",
				logger,
				managementLibLogger,
			)
			if err != nil {
				return err
			}

			// get flattened config maps
			map1 := conf1.GetIntermediateConfig()
			map2 := conf2.GetIntermediateConfig()

			diffs := diffFlatMaps(
				map1,
				map2,
			)

			if len(diffs) > 0 {
				fmt.Println(strings.Join(diffs, ""))
				return errConfigsDiffer
			}

			return nil
		},
	}

	return res
}

// diffFlatMaps reports differences between flattened config maps
// this only works for maps 1 layer deep as produced by the management
// lib's flattenConf function
func diffFlatMaps(m1 map[string]any, m2 map[string]any) []string {
	var res []string

	allKeys := map[string]struct{}{}
	for k := range m1 {
		allKeys[k] = struct{}{}
	}

	for k := range m2 {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		// "index" is a metadata key added by
		// the management lib to these flat maps
		// ignore it
		if strings.HasSuffix(k, ".index") {
			continue
		}

		v1, ok := m1[k]
		if !ok {
			res = append(res, fmt.Sprintf("\n\t-: %s\n", k))
			continue
		}

		v2, ok := m2[k]
		if !ok {
			res = append(res, fmt.Sprintf("\n\t+: %s\n", k))
			continue
		}

		if !reflect.DeepEqual(v1, v2) {
			res = append(res, fmt.Sprintf("\n%s:\n\t-: %v\n\t+: %v\n", k, v1, v2))
		}
	}

	return res
}
