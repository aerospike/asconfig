package cmd

import (
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
				against a versioned Aerospike configuration JSON schema.
				If a file passes validation nothing is output, otherwise errors
				indicating problems with the configuration file are shown.
				If a file path is not provided, validate reads from stdin.
				Ex: asconfig validate --aerospike-version 7.0.0 aerospike.conf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running validate command")

			if len(args) > convertArgMax {
				return errValidateTooManyArguments
			}

			// read stdin by default
			var srcPath string
			if len(args) == 0 {
				srcPath = os.Stdin.Name()
			} else {
				srcPath = args[0]
			}

			version, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag aerospike-version value=%s", version)

			srcFormat, err := getConfFileFormat(srcPath, cmd)
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag format value=%v", srcFormat)

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

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

			// TODO should the validation errors be printed to stdout or stderr?
			// should they come with the standard error log line?
			err = conf.Validate()
			if err != nil {
				return err
			}

			return err
		},
	}

	// flags and configuration settings
	// aerospike-version is marked required in this cmd's PreRun if the --force flag is not provided
	res.Flags().StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.\nThe first 3 digits of the Aerospike version number are required.\nThis option is required unless --force is used")
	res.MarkFlagRequired("aerospike-version")

	res.Version = VERSION

	return res
}
