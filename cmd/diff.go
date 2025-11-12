package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	lib "github.com/aerospike/aerospike-management-lib"
	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/spf13/cobra"

	"github.com/aerospike/asconfig/conf"
	"github.com/aerospike/asconfig/schema"
)

const (
	diffArgMin         = 2
	diffArgMax         = 2
	diffServerArgMin   = 1 // For server diff, we need only one local file
	diffServerArgMax   = 1
	diffVersionsArgMin = 2 // For versions diff, we need exactly 2 versions
	diffVersionsArgMax = 2
)

// GetDiffCmd returns the diff command.
func newDiffCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "diff",
		Short: "Diff Aerospike configuration files or a file against a running server's configuration.",
		Long: `Diff is used to compare Aerospike configuration files, or a file against a running server's configuration.
				
				If no subcommand is provided, 'files' is used by default for backward compatibility.

				See subcommands for available diff modes.`,
		Example: `
				# Diff two local yaml configuration files
				asconfig diff files aerospike1.yaml aerospike2.yaml
				# Diff a local .conf file against a running server
				asconfig diff server -h 127.0.0.1:3000  aerospike.conf
				# Compare configuration changes between versions
				asconfig diff versions 7.0.0 8.1.0
				# Compare configuration changes between versions and focus on specific configuration areas
				asconfig diff versions 7.0.0 8.0.0 --filter-path "logging,namespaces"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Warn("Using legacy 'diff' subcommand. Use 'diff files' instead.")
			return runFileDiff(cmd, args)
		},
	}

	res.Version = VERSION
	res.Flags().
		StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

	// Add subcommands
	res.AddCommand(newDiffFilesCmd())
	res.AddCommand(newDiffServerCmd())
	res.AddCommand(newDiffVersionsCmd())

	return res
}

// newDiffFilesCmd creates the 'diff files' subcommand (the legacy default).
func newDiffFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files [flags] <path/to/config1> <path/to/config2>",
		Short: "Diff yaml or conf Aerospike configuration files.",
		Long: `Diff is used to compare differences between two Aerospike configuration files.
			It is used on two files of the same format from any format
			supported by the asconfig tool, e.g. yaml or Aerospike config.
			Schema validation is not performed on either file. The file names must end with
			extensions signifying their formats, e.g. .conf or .yaml, or --format must be used.`,
		Example: `
			# Compare two local configuration files
  				asconfig diff files aerospike1.conf aerospike2.conf
			# Compare two local yaml configuration files
				asconfig diff files --format yaml aerospike1.yaml aerospike2.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running diff files command")
			return runFileDiff(cmd, args)
		},
	}
	cmd.Version = VERSION
	cmd.Flags().
		StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

	return cmd
}

func newDiffServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server [flags] <path/to/config>",
		Short: "BETA: Diff a local config file against a running Aerospike server's configuration.",
		Long: `BETA: Diff is used to compare a local configuration file against the configuration of a running Aerospike server. 
				This is useful for spotting drift between expected and actual Aerospike server configurations.
				In this mode, only one config file path is required as an argument.
				Note: The configuration file can be in yaml or conf format.`,
		Example: `Diff a local .conf file against a running server
  				asconfig diff server -h 127.0.0.1:3000 aerospike.conf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running server diff command")
			return runServerDiff(cmd, args)
		},
	}

	// Add format flag but hide it from help output as it will be automatically detected
	cmd.Flags().
		StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

	if err := cmd.Flags().MarkHidden("format"); err != nil {
		logger.Errorf("Unable to hide format flag: %v", err)
		// this is appropriate since this function returns a *cobra.Command
		// and we can't proceed with command creation if flag setup fails
		return nil
	}

	// Add Aerospike connection flags when server mode is enabled
	asFlagSet := aerospikeFlags.NewFlagSet(flags.DefaultWrapHelpString)
	cmd.Flags().AddFlagSet(asFlagSet)
	config.BindPFlags(asFlagSet, "cluster")
	cmd.Version = VERSION

	return cmd
}

func newDiffVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions [flags] <version1> <version2>",
		Short: "Show configuration file difference between versions of the Aerospike server.",
		Long: `Compare configuration schemas between two Aerospike server versions to understand
			what changes when upgrading or downgrading. This command shows which configuration 
			parameters are added, removed, or modified between versions in a detailed, human-readable format.

			By default, detailed information is shown including property types, defaults, and descriptions.
			Use --compact to show only configuration names for a minimal view.
			Use --filter-path to focus on specific configuration sections.`,
		Example: `
			# Compare configuration changes between versions (detailed by default)
			asconfig diff versions 7.0.0 7.2.0
			asconfig diff versions 8.1.0 7.0.0  # automatically reordered

			# Show minimal output with only configuration names
			asconfig diff versions 6.4.0 7.0.0 --compact

			# Focus on specific configuration areas
			asconfig diff versions 7.0.0 8.0.0 --filter-path "logging,namespaces"

			# Combine compact view with filtering
			asconfig diff versions 7.0.0 8.0.0 --compact --filter-path "service"

			# List all available Aerospike server versions
			asconfig list versions --verbose
			`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running versions diff command")
			return runVersionsDiff(cmd, args)
		},
	}

	cmd.Flags().
		BoolP("compact", "c", false, "Show minimal output with only configuration names (default shows detailed information)")
	cmd.Flags().
		StringP("filter-path", "f", "", "Filter results to only show properties under the specified path (e.g., 'service', 'namespaces')")
	cmd.Version = VERSION

	return cmd
}

// runFileDiff handles the original file-to-file diff functionality.
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
		return fmt.Errorf("%w: detected %s and %s", errMismatchedFileFormats, fmt1, fmt2)
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
		fmt.Fprintf(
			os.Stdout,
			"Differences shown from %s to %s, '<' are from file1, '>' are from file2.\n",
			path1,
			path2,
		)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(diffs, ""))

		return fmt.Errorf("%w: %w", errDiffConfigsDiffer, ErrSilent)
	}

	return nil
}

// runServerDiff handles comparing a local file against a running server.
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

	logger.Debugf("Local file format is %v", localFormat)

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
		return fmt.Errorf("%w: %w", errUnableToCreateClientPolicy, err)
	}

	logger.Debugf("Retrieving Aerospike configuration from server")

	asHosts := asCommonConfig.NewHosts()
	asinfo := info.NewAsInfo(mgmtLibLogger, asHosts[0], asPolicy)

	generatedConf, err := asConf.GenerateConf(mgmtLibLogger, asinfo, true)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnableToGenerateConfigFromServer, err)
	}

	// Convert server config to the same format as local file to ensure same parsing path
	serverConfHandler, err := asConf.NewMapAsConfig(mgmtLibLogger, generatedConf.Conf)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnableToParseGeneratedServerConf, err)
	}

	// Marshal server config to bytes in the same format as local file
	serverConfigMarshaller := conf.NewConfigMarshaller(serverConfHandler, localFormat)

	serverConfigBytes, err := serverConfigMarshaller.MarshalText()
	if err != nil {
		return fmt.Errorf("%w: %w", errUnableToMarshalServerConfig, err)
	}

	// Parse server config bytes using the same path as local file
	serverConf, err := asConf.NewASConfigFromBytes(mgmtLibLogger, serverConfigBytes, localFormat)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnableToParseServerConfigBytes, err)
	}

	// Get flattened config maps - now both should have the same data types
	localMap := localConf.GetFlatMap()
	serverMap := serverConf.GetFlatMap()

	diffs := diffFlatMaps(
		*localMap,
		*serverMap,
	)

	if len(diffs) > 0 {
		fmt.Fprintf(
			os.Stdout,
			"Differences shown from %s to server, '<' are from local file, '>' are from server.\n",
			localPath,
		)
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(diffs, ""))

		return fmt.Errorf("%w: %w", errDiffConfigsDiffer, ErrSilent)
	}

	return nil
}

// runVersionsDiff compares the configuration between two Aerospike server versions.
func runVersionsDiff(cmd *cobra.Command, args []string) error {
	if len(args) < diffVersionsArgMin {
		return errSchemaDiffWrongArgs
	}

	if len(args) > diffVersionsArgMax {
		return errSchemaDiffWrongArgs
	}

	version1 := args[0]
	version2 := args[1]

	// Use lib.CompareVersions to determine order and auto-reverse if needed
	compareResult, err := lib.CompareVersions(version1, version2)
	if err != nil {
		return fmt.Errorf("failed to compare versions %s and %s: %w", version1, version2, err)
	}

	// If version1 > version2 (compareResult > 0), swap them for logical diff order
	if compareResult > 0 {
		logger.Debugf(
			"Reversing version order: %s > %s, showing diff from %s to %s",
			version1,
			version2,
			version2,
			version1,
		)
		version1, version2 = version2, version1
	}

	logger.Debugf("Comparing schema from version %s to version %s", version1, version2)

	// Load schemas
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		return fmt.Errorf("failed to load schema map: %w", err)
	}

	schema1, exists := schemaMap[version1]
	if !exists {
		return errors.Join(errInvalidSchemaVersion, fmt.Errorf("schema for version %s not found", version1))
	}

	schema2, exists := schemaMap[version2]
	if !exists {
		return errors.Join(errInvalidSchemaVersion, fmt.Errorf("schema for version %s not found", version2))
	}

	var schemaLower, schemaUpper map[string]any
	if unmarshalErr := json.Unmarshal([]byte(schema1), &schemaLower); unmarshalErr != nil {
		return fmt.Errorf("failed to parse schema for version %s: %w", version1, unmarshalErr)
	}
	if unmarshalErr := json.Unmarshal([]byte(schema2), &schemaUpper); unmarshalErr != nil {
		return fmt.Errorf("failed to parse schema for version %s: %w", version2, unmarshalErr)
	}

	// Get flags - verbose is now the default, compact is the exception
	compact, _ := cmd.Flags().GetBool("compact")
	verbose := !compact // Verbose is the default behavior, compact overrides it
	filterPath, _ := cmd.Flags().GetString("filter-path")

	filterSections := make(map[string]struct{})
	if filterPath != "" {
		sections := strings.Split(filterPath, ",")
		for _, s := range sections {
			filterSections[strings.TrimSpace(s)] = struct{}{}
		}
	}

	// Compare the two JSON files
	summary, err := compareSchemas(schemaLower, schemaUpper, version1, version2)
	if err != nil {
		return fmt.Errorf("failed to compare schemas: %w", err)
	}

	// Validate filter sections if provided
	if len(filterSections) > 0 {
		if validFilterErr := validateFilterSections(filterSections, summary.Sections); validFilterErr != nil {
			return validFilterErr
		}
	}

	// Output the results
	printChangeSummary(summary, DiffOptions{
		Verbose:        verbose,
		FilterSections: filterSections,
	})

	return nil
}

// diffFlatMaps reports differences between flattened config maps
// this only works for maps 1 layer deep as produced by the management
// lib's flattenConf function.
func diffFlatMaps(m1, m2 map[string]any) []string {
	var res []string

	allKeys := map[string]struct{}{}
	for k := range m1 {
		allKeys[k] = struct{}{}
	}

	for k := range m2 {
		allKeys[k] = struct{}{}
	}

	keysList := make([]string, 0, len(allKeys))
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
