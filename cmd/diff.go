package cmd

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/asconfig/conf"
	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"

	"github.com/spf13/cobra"
)

const (
	diffArgMin       = 2
	diffArgMax       = 2
	diffServerArgMin = 1 // For server diff, we need only one local file
	diffServerArgMax = 1
)

var (
	errDiffConfigsDiffer     = errors.Join(fmt.Errorf("configuration files are not equal"), ErrSilent)
	errDiffTooFewArgs        = fmt.Errorf("diff requires atleast %d file paths as arguments", diffArgMin)
	errDiffTooManyArgs       = fmt.Errorf("diff requires no more than %d file paths as arguments", diffArgMax)
	errDiffServerTooFewArgs  = fmt.Errorf("diff with --server requires exactly %d file path as argument", diffServerArgMin)
	errDiffServerTooManyArgs = fmt.Errorf("diff with --server requires no more than %d file path as argument", diffServerArgMax)
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
		Example: `  # Compare two local configuration files
				  asconfig diff aerospike1.yaml aerospike2.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running diff command")
			return runFileDiff(cmd, args)
		},
	}

	res.Version = VERSION
	res.PersistentFlags().StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

	// Add server diff command
	res.AddCommand(newDiffServerCmd())

	return res
}

func newDiffServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server [flags] <path/to/config>",
		Short: "Compare a local config file against a running Aerospike server.",
		Long: `Compare a local configuration file against the live configuration from a running Aerospike server. In this mode,
only one config file path is required as an argument.`,
		Example: `  # Compare a local .conf file against a running server
  asconfig diff server -h 127.0.0.1 -U admin -P admin aerospike.conf
  
  # Compare a local .yaml file against a running server
  asconfig diff server --format yaml -h 127.0.0.1 -U admin -P admin aerospike.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running server diff command")
			return runServerDiff(cmd, args)
		},
	}

	// Add Aerospike connection flags when server mode is enabled
	asFlagSet := aerospikeFlags.NewFlagSet(flags.DefaultWrapHelpString)
	cmd.Flags().AddFlagSet(asFlagSet)
	config.BindPFlags(asFlagSet, "cluster")

	return cmd
}

// runFileDiff handles the original file-to-file diff functionality
func runFileDiff(cmd *cobra.Command, args []string) error {
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
	conf1, err := asConf.NewASConfigFromBytes(mgmtLibLogger, f1, fmt1)
	if err != nil {
		return err
	}

	conf2, err := asConf.NewASConfigFromBytes(mgmtLibLogger, f2, fmt2)
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
}

// runServerDiff handles comparing a local file against a running server
func runServerDiff(cmd *cobra.Command, args []string) error {
	if len(args) < diffServerArgMin {
		return errDiffServerTooFewArgs
	}

	if len(args) > diffServerArgMax {
		return errDiffServerTooManyArgs
	}

	logger.Warning(
		"This feature is currently in beta. Use at your own risk and please report any issue to support.",
	)

	localPath := args[0]
	logger.Debugf("Comparing local file %s against server", localPath)

	// Get local file format and content
	localFormat, err := getConfFileFormat(localPath, cmd)
	if err != nil {
		return err
	}

	localFile, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	// Create local config
	localConf, err := asConf.NewASConfigFromBytes(mgmtLibLogger, localFile, localFormat)
	if err != nil {
		return err
	}

	logger.Debugf("Generating config from Aerospike node")

	// Generate server config using existing generate functionality
	asCommonConfig := aerospikeFlags.NewAerospikeConfig()

	asPolicy, err := asCommonConfig.NewClientPolicy()
	if err != nil {
		return errors.Join(fmt.Errorf("unable to create client policy"), err)
	}

	logger.Debugf("Retrieving Aerospike configuration from server")

	asHosts := asCommonConfig.NewHosts()
	asinfo := info.NewAsInfo(mgmtLibLogger, asHosts[0], asPolicy)

	generatedConf, err := asConf.GenerateConf(mgmtLibLogger, asinfo, true)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to generate config from server"), err)
	}

	// Convert server config to the same format as local file to ensure same parsing path
	serverConfHandler, err := asConf.NewMapAsConfig(mgmtLibLogger, generatedConf.Conf)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to parse the generated server conf"), err)
	}

	// Marshal server config to bytes in the same format as local file
	serverConfigMarshaller := conf.NewConfigMarshaller(serverConfHandler, localFormat)
	serverConfigBytes, err := serverConfigMarshaller.MarshalText()
	if err != nil {
		return errors.Join(fmt.Errorf("unable to marshal server config"), err)
	}

	// Parse server config bytes using the same path as local file
	serverConf, err := asConf.NewASConfigFromBytes(mgmtLibLogger, serverConfigBytes, localFormat)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to parse server config bytes"), err)
	}

	// Get flattened config maps - now both should have the same data types
	localMap := localConf.GetFlatMap()
	serverMap := serverConf.GetFlatMap()

	diffs := diffFlatMaps(
		*localMap,
		*serverMap,
	)

	if len(diffs) > 0 {
		fmt.Printf("Differences shown from %s to server, '<' are from local file, '>' are from server.\n", localPath)
		fmt.Println(strings.Join(diffs, ""))
		return errDiffConfigsDiffer
	}

	return nil
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

		// #TOOLS-2979 if part of logging section and is valid logging enum when compared "info" == "INFO"
		if strings.HasPrefix(k, "logging.") && isValidLoggingEnumCompare(v1, v2) {
			continue
		}

		if !reflect.DeepEqual(v1, v2) {
			// Debug: print types and values for investigation
			logger.Debugf("Diff found for key '%s': local=%v (type=%T), server=%v (type=%T)", k, v1, v1, v2, v2)
			res = append(res, fmt.Sprintf("%s:\n\t<: %v\n\t>: %v\n", k, v1, v2))
		}
	}

	return res
}
