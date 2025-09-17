package cmd

import (
	"errors"
	"fmt"
	"os"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"

	"github.com/aerospike/asconfig/conf"
)

const (
	validateArgMax = 1
)

var (
	ErrValidateTooManyArguments = fmt.Errorf("expected a maximum of %d arguments", convertArgMax)
)

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
		RunE: runValidateCommand,
	}

	// flags and configuration settings
	// --aerospike-version is required unless the server version
	// is in the input config file's metadata
	commonFlags := getCommonFlags()
	res.Flags().AddFlagSet(commonFlags)
	res.Flags().
		StringP("format", "F", "conf", "The format of the source file(s). Valid options are: yaml, yml, and conf.")

	res.Version = VERSION

	return res
}

// runValidateCommand handles the main validation logic.
func runValidateCommand(cmd *cobra.Command, args []string) error {
	logger.Debug("Running validate command")

	if len(args) > validateArgMax {
		return ErrValidateTooManyArguments
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
		return errors.Join(ErrMissingAerospikeVersion, err)
	}

	// if the Aerospike server version was not in the file
	// metadata, require that it is passed as an argument
	if version == "" {
		err = cmd.MarkFlagRequired("aerospike-version")
		if err != nil {
			return err
		}
	}

	versionArg, err := cmd.Flags().GetString("aerospike-version")
	if err != nil {
		logger.Errorf("Unable to get aerospike-version flag: %v", err)
		return err
	}

	// the command line --aerospike-version option overrides
	// the metadata server version
	if versionArg != "" {
		version = versionArg
	}

	logger.Debugf("Processing flag aerospike-version value=%s", version)

	asconfig, err := asConf.NewASConfigFromBytes(mgmtLibLogger, fdata, srcFormat)

	if err != nil {
		return err
	}

	verrs, err := conf.NewConfigValidator(asconfig, mgmtLibLogger, version).Validate()
	// verrs is an empty slice if err is not nil but no
	// validation errors were found
	if verrs != nil && len(verrs.Errors) > 0 {
		cmd.Print(verrs.Error())
		return errors.Join(conf.ErrConfigValidation, ErrSilent)
	}

	return err
}
