package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	convertArgMin = 1
	convertArgMax = 1
)

var (
	errNotEnoughArguments          = fmt.Errorf("expected a minimum of %d arguments", convertArgMin)
	errTooManyArguments            = fmt.Errorf("expected a maximum of %d arguments", convertArgMax)
	errFileNotExist                = fmt.Errorf("file does not exist")
	errInvalidAerospikeVersion     = fmt.Errorf("aerospike version must be in the form <a>.<b>.<c>")
	errUnsupportedAerospikeVersion = fmt.Errorf("aerospike version unsupported")
	errConfigValidation            = fmt.Errorf("error while validating aerospike config")
	errInvalidOutput               = fmt.Errorf("invalid output flag")
	errInvalidFormat               = fmt.Errorf("invalid format flag")
)

func init() {
	rootCmd.AddCommand(convertCmd)
}

var convertCmd = newConvertCmd()

func newConvertCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "convert [flags] <path/to/config.yaml>",
		Short: "Convert yaml to aerospike config format.",
		Long: `Convert is used to convert between yaml and aerospike configuration
				files. In the future, this command may be able to convert from asconf back to yaml.
				Specifying the server version that will use the aerospike.conf is required.
				Usage examples...
				convert local file "aerospike.yaml" to aerospike config format for version 6.2.0 and
				write it to local file "aerospike.conf."
				EX: asconfig convert --aerospike-version "6.2.0" aerospike.yaml --output aerospike.conf
				Short form flags and source file only conversions are also supported.
				In this case, -a is the server version and using only a source file means
				the result will be written to stdout.
				EX: asconfig convert -a "6.2.0" aerospike.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running convert command")

			srcPath := args[0]

			version, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag aerospike-version value=%s", version)

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag force value=%t", force)

			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag format value=%s", format)

			logger.Debug("Processing source file")

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			var asConf *asconfig.AsConfig
			var out []byte

			if format == "yaml" {
				var data map[string]any
				err = yaml.Unmarshal(fdata, &data)
				if err != nil {
					return err
				}

				asConf, err = asconfig.NewMapAsConfig(managementLibLogger, version, data)
				if err != nil {
					return fmt.Errorf("failed to initialize asconfig from yaml: %w", err)
				}

				out = []byte(asConf.ToConfFile())
			} else if format == "asconfig" {
				reader := bytes.NewReader(fdata)

				asConf, err = asconfig.FromConfFile(managementLibLogger, version, reader)
				if err != nil {
					return fmt.Errorf("failed to parse asconfig file: %w", err)
				}

				out, err = yaml.Marshal(asConf.ToMap())
				if err != nil {
					return fmt.Errorf("failed to marshal asconfig to yaml: %w", err)
				}

			} else {
				return fmt.Errorf("%w %s", errInvalidFormat, format)
			}

			if !force {
				valid, validationErrors, err := asConf.IsValid(managementLibLogger, version)
				if !valid {
					logger.Errorf("Invalid aerospike configuration file: %s", srcPath)
				}
				if len(validationErrors) > 0 {
					for _, e := range validationErrors {
						logger.Errorf("Aerospike config validation error: %+v", e)
					}
				}
				if !valid || err != nil {
					return fmt.Errorf("%w, %w", errConfigValidation, err)
				}
			}

			outputPath, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}

			if stat, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
				// output path is a directory so write a new file to it
				srcFileName := filepath.Base(srcPath)
				srcFileName = strings.TrimSuffix(srcFileName, filepath.Ext(srcFileName))

				outputPath = filepath.Join(outputPath, srcFileName)
				outputPath += ".conf"
			}

			var outFile *os.File
			if outputPath == os.Stdout.Name() {
				outFile = os.Stdout
			} else {
				outFile, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					return err
				}

				defer outFile.Close()
			}

			logger.Debugf("Writing converted data to: %s", outputPath)
			_, err = outFile.Write([]byte(out))
			return err

		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var multiErr error

			// validate arguments
			if len(args) < convertArgMin {
				logger.Errorf("Expected atleast %d argument(s)", convertArgMin)
				// multiErr = errors.Join(multiErr, errNotEnoughArguments) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, errNotEnoughArguments)
			}

			//TODO validate the --format flag

			if len(args) > convertArgMax {
				logger.Errorf("Expected no more than %d argument(s)", convertArgMax)
				// multiErr = errors.Join(multiErr, errTooManyArguments) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, errTooManyArguments)
			}

			if len(args) > 0 {
				source := args[0]
				if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
					logger.Errorf("Source file does not exist %s", source)
					// multiErr = errors.Join(multiErr, errFileNotExist, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %w %w", multiErr, errFileNotExist, err)
				}
			}

			// validate flags
			_, err := cmd.Flags().GetString("output")
			if err != nil {
				// multiErr = errors.Join(multiErr, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				// multiErr = errors.Join(multiErr, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			if !force {
				cmd.MarkFlagRequired("aerospike-version")
			}

			av, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			if !force {
				if av == "" {
					logger.Error("missing required flag '--aerospike-version'")
					// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, missing required flag '--aerospike-version' %w", multiErr, errInvalidAerospikeVersion)
				}

				supported, err := asconfig.IsSupportedVersion(av)
				if err != nil {
					logger.Errorf("Failed to check aerospike version %s for compatibility", av)
					// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %w %w", multiErr, errInvalidAerospikeVersion, err)
				}

				// TODO include valid versions in the error message
				// asconfig lib needs a getSupportedVersions func
				if !supported {
					logger.Errorf("Unsupported aerospike server version: %s", av)
					// multiErr = errors.Join(multiErr, errUnsupportedAerospikeVersion) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %s %w", multiErr, av, errUnsupportedAerospikeVersion)
				}
			}

			return multiErr
		},
	}

	// flags and configuration settings
	// aerospike-version is marked required in this cmd's PreRun if the --force flag is not provided
	res.Flags().StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.\nThe first 3 digits of the Aerospike version number are required.\nThis option is required unless --force is used")
	res.Flags().BoolP("force", "f", false, "Override checks for supported server version and config validation")
	res.Flags().StringP("output", "o", os.Stdout.Name(), "File path to write output to")
	res.Flags().StringP("format", "F", "asconfig", "The format to convert the source file to. Valid options are: yaml, asconfig")

	res.Version = VERSION

	return res
}
