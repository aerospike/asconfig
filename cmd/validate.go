package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/aerospike/asconfig/asconf"
	"github.com/spf13/cobra"
)

const (
	validateArgMax = 1
)

var (
	errValidateTooManyArguments = fmt.Errorf("expected a maximum of %d arguments", convertArgMax)
)

func init() {
	rootCmd.AddCommand(validateCmd)
}

var validateCmd = newValidateCmd()

func newValidateCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "validate [flags] <path/to/config_file>",
		Short: "Validate an Aerospike configuration file.",
		Long: `Validate an Aerospike configuration file in any supported format
				against a versioned Aerospike configuration schema.
				If a file passes validation nothing is output, otherwise errors
				indicating problems with the configuration file are shown.
				If a file path is not provided, validate reads from stdin.
				Ex: asconfig validate --aerospike-version 7.0.0 aerospike.conf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running validate command")

			if len(args) > validateArgMax {
				return errValidateTooManyArguments
			}

			// read stdin by default
			var srcPath string
			if len(args) == 0 {
				srcPath = os.Stdin.Name()
			} else {
				srcPath = args[0]
			}

			srcFormat, err := getConfFileFormat(srcPath, cmd)
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag format value=%v", srcFormat)

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			version, err := getMetaDataItemOptional(fdata, metaKeyAerospikeVersion)
			if err != nil {
				return errors.Join(errMissingAerospikeVersion, err)
			}

			// if the Aerospike server version was not in the file
			// metadata, require that it is passed as an argument
			if version == "" {
				cmd.MarkFlagRequired("aerospike-version")
			}

			versionArg, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}

			// the command line --aerospike-version option overrides
			// the metadata server version
			if versionArg != "" {
				version = versionArg
			}

			logger.Debugf("Processing flag aerospike-version value=%s", version)

			conf, err := asconf.NewAsconf(
				fdata,
				srcFormat,
				// we aren't converting to anything so set
				// output format to Invalid as a place holder
				asconf.Invalid,
				version,
				logger,
				managementLibLogger,
			)

			if err != nil {
				return err
			}

			verrs, err := conf.Validate()
			if verrs != nil {
				// force validation errors to be written to stdout
				// so they can more easily be grepd etc.
				cmd.Print(verrs.Error())
				return errors.Join(asconf.ErrConfigValidation, SilentError)
			}
			if err != nil {
				return err
			}

			return err
		},
	}

	// flags and configuration settings
	// --aerospike-version is required unless the server version
	// is in the input config file's metadata
	commonFlags := getCommonFlags()
	res.Flags().AddFlagSet(commonFlags)

	res.Version = VERSION

	return res
}
