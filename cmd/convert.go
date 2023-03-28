package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	argMin = 1
	argMax = 2
)

var (
	errNotEnoughArguments          = fmt.Errorf("expected a minimum of %d arguments", argMin)
	errTooManyArguments            = fmt.Errorf("expected a maximum of %d arguments", argMax)
	errFileNotExist                = fmt.Errorf("file does not exist")
	errFileisDir                   = fmt.Errorf("file is a directory")
	errInvalidAerospikeVersion     = fmt.Errorf("aerospike version must be in the form <a>.<b>.<c>")
	errUnsupportedAerospikeVersion = fmt.Errorf("aerospike version unsupported")
	errConfigValidation            = fmt.Errorf("error while validating aerospike config")
)

func init() {
	rootCmd.AddCommand(convertCmd)
}

var convertCmd = newConvertCmd()

func newConvertCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "convert <path/to/config.yaml> [<path/to/aerospike.conf>]",
		Short: "Convert yaml to aerospike config format.",
		Long: `Convert is used to convert between yaml and aerospike configuration
				files. In the future, this command may be able to convert from asconf back to yaml.
				Specifying the server version that will use the aerospike.conf is required.
				Usage examples...
				convert local file "aerospike.yaml" to aerospike config format for version 6.2.0.2 and
				write it to local file "aerospike.conf."
				EX: asconfig convert --aerospike-version "6.2.0.2" aerospike.yaml aerospike.conf
				Short form flags and source file only conversions are also supported.
				In this case, -a is the server version and using only a source file means
				the result will be written as <path/to/config>.conf
				EX: asconfig convert -a "6.2.0.2 aerospike.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running convert command")

			srcPath := args[0]
			logger.Debug("Processing source file")

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

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			var data map[string]any
			err = yaml.Unmarshal(fdata, &data)
			if err != nil {
				return err
			}

			asConf, err := asconfig.NewMapAsConfig(managementLibLogger, version, data)
			if err != nil {
				return fmt.Errorf("failed to initialize AsConfig from yaml: %w", err)
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

			confFile := asConf.ToConfFile()

			// TODO asconf to yaml

			destPath := os.Stdout.Name()
			if len(args) > argMin {
				destPath = args[1]
				logger.Debugf("Processing output file %s", destPath)
			}

			logger.Debugf("Writing converted data to: %s", destPath)
			return os.WriteFile(destPath, []byte(confFile), 0644)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var multiErr error

			// validate arguments
			if len(args) < argMin {
				logger.Errorf("Expected atleast %d arguments", argMin)
				// multiErr = errors.Join(multiErr, errNotEnoughArguments) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, errNotEnoughArguments)
			}

			if len(args) > argMax {
				logger.Errorf("Expected no more than %d arguments", argMax)
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

			if len(args) > argMin {
				dest := args[1]
				if stat, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
					logger.Errorf("Output file is a directory %s", dest)
					// multiErr = errors.Join(multiErr, errFileisDir, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %s %w", multiErr, dest, errFileisDir)
				}
			}

			// validate flags
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				// multiErr = errors.Join(multiErr, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			av, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			if !force {
				supported, err := asconfig.IsSupportedVersion(av)
				if err != nil {
					logger.Errorf("Failed to check %s for compatibility", av)
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
	res.PersistentFlags().StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.2")
	res.MarkPersistentFlagRequired("aerospike-version")
	res.PersistentFlags().BoolP("force", "f", false, "Override checks for supported server version and config validation")

	res.Version = VERSION

	return res
}
