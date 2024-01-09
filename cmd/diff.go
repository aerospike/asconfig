package cmd

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/aerospike/asconfig/conf"

	"github.com/spf13/cobra"
)

const (
	diffArgMin = 2
	diffArgMax = 2
)

var (
	errDiffConfigsDiffer = errors.Join(fmt.Errorf("configuration files are not equal"), ErrSilent)
	errDiffTooFewArgs    = fmt.Errorf("diff requires atleast %d file paths as arguments", diffArgMin)
	errDiffTooManyArgs   = fmt.Errorf("diff requires no more than %d file paths as arguments", diffArgMax)
)

func init() {
	rootCmd.AddCommand(diffCmd)
}

var diffCmd = newDiffCmd()

func newDiffCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "diff [flags] <path/to/config1> <path/to/config2>",
		Short: "Diff yaml or conf Aerospike configuration files.",
		Long: `Diff is used to compare differences between Aerospike configuration files.
				It is used on two files of the same format from any format
				supported by the asconfig tool, e.g. yaml or Aerospike config.
				Schema validation is not performed on either file. The file names must end with
				extensions signifying their formats, e.g. .conf or .yaml, or --format must be used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running diff command")

			if len(args) < diffArgMin {
				return errDiffTooFewArgs
			}

			if len(args) > diffArgMax {
				return errDiffTooManyArgs
			}

			path1 := args[0]
			path2 := args[1]

			logger.Debugf("Diff file 1 is %s", path1)
			logger.Debugf("Diff file 2 is %s", path2)

			fmt1, err := getConfFileFormat(path1, cmd)
			if err != nil {
				return err
			}

			fmt2, err := getConfFileFormat(path2, cmd)
			if err != nil {
				return err
			}

			logger.Debugf("Diff file 1 format is %v", fmt1)
			logger.Debugf("Diff file 2 format is %v", fmt2)

			if fmt1 != fmt2 {
				return fmt.Errorf("mismatched file formats, detected %s and %s", fmt1, fmt2)
			}

			f1, err := os.ReadFile(path1)
			if err != nil {
				return err
			}

			f2, err := os.ReadFile(path2)
			if err != nil {
				return err
			}

			// not performing any validation so server version is "" (not needed)
			// won't be marshaling these configs to text so use Invalid output format
			// TODO decouple output format from asconf, probably pass it as an
			// arg to marshal text
			conf1, err := conf.NewASConfigFromBytes(mgmtLibLogger, f1, fmt1)
			if err != nil {
				return err
			}

			conf2, err := conf.NewASConfigFromBytes(mgmtLibLogger, f2, fmt2)
			if err != nil {
				return err
			}

			// get flattened config maps
			map1 := conf1.GetFlatMap()
			map2 := conf2.GetFlatMap()

			diffs := diffFlatMaps(
				*map1,
				*map2,
			)

			if len(diffs) > 0 {
				fmt.Printf("Differences shown from %s to %s, '<' are from file1, '>' are from file2.\n", path1, path2)
				fmt.Println(strings.Join(diffs, ""))
				return errDiffConfigsDiffer
			}

			return nil
		},
	}

	res.Flags().StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

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

	var keysList []string
	for k := range allKeys {
		keysList = append(keysList, k)
	}
	sort.Strings(keysList)

	for _, k := range keysList {
		// "index" is a metadata key added by
		// the management lib to these flat maps
		// ignore it
		if strings.HasSuffix(k, ".<index>") {
			continue
		}

		v1, ok := m1[k]
		if !ok {
			res = append(res, fmt.Sprintf(">: %s\n", k))
			continue
		}

		v2, ok := m2[k]
		if !ok {
			res = append(res, fmt.Sprintf("<: %s\n", k))
			continue
		}

		if !reflect.DeepEqual(v1, v2) {
			res = append(res, fmt.Sprintf("%s:\n\t<: %v\n\t>: %v\n", k, v1, v2))
		}
	}

	return res
}
