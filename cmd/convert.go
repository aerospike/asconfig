package cmd

import (
	"errors"
	"fmt"
	"os"

	"aerospike/asconfig/log"

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
			logger.Info("Running command", log.Command, "convertCmd")

			srcPath := args[0]
			logger.Info("Processing argument", log.Argument, "source", log.Value, srcPath)

			version, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}
			logger.Info("Processing flag", log.Flag, "aerospike-version", log.Value, version)

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}
			logger.Info("Processing flag", log.Flag, "force", log.Value, force)

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			var data map[string]any
			err = yaml.Unmarshal(fdata, &data)
			if err != nil {
				return err
			}

			asConf, err := asconfig.NewMapAsConfig(logger, version, data)
			if err != nil {
				return fmt.Errorf("failed to initialize AsConfig from yaml: %v", err)
			}

			if !force {
				valid, validationErrors, err := asConf.IsValid(logger, version)
				if !valid {
					logger.Error(errConfigValidation, "Valid is false", log.File, srcPath)
				}
				if len(validationErrors) > 0 {
					for _, e := range validationErrors {
						errorKeysAndValues, err := log.StructToKeysAndValues(*e)
						if err != nil {
							return err
						}

						keysAndValues := append([]any{log.File, srcPath}, errorKeysAndValues...)
						logger.Error(errConfigValidation, "validationErrors", keysAndValues...)
					}
				}
				if err != nil {
					return err
				}
			}

			confFile := asConf.ToConfFile()

			// TODO asconf to yaml

			destPath := os.Stdout.Name()
			if len(args) > argMin {
				destPath = args[1]
				logger.Info("Processing argument", log.Argument, "destination", log.Value, destPath)
			}

			logger.Info("Writing converted data to", log.File, destPath)
			return os.WriteFile(destPath, []byte(confFile), 0644)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var multiErr error

			// validate arguments
			if len(args) < argMin {
				logger.Error(errNotEnoughArguments, "Too few arguments", log.Command, "convertCmd", log.Count, len(args), log.Expected, argMin)
				// multiErr = errors.Join(multiErr, errNotEnoughArguments) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, errNotEnoughArguments)
				return multiErr
			}

			if len(args) > argMax {
				logger.Error(errTooManyArguments, "Too many arguments", log.Command, "convertCmd", log.Count, len(args), log.Expected, argMax)
				// multiErr = errors.Join(multiErr, errTooManyArguments) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, errTooManyArguments)
			}

			source := args[0]
			if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
				logger.Error(errFileNotExist, "Source file does not exist", log.Command, "convertCmd", log.File, source)
				// multiErr = errors.Join(multiErr, errFileNotExist, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w, %w", multiErr, errFileNotExist, err)
			}

			if len(args) > argMin {
				dest := args[1]
				if stat, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
					logger.Error(errFileisDir, "File to write to is a directory", log.Command, "convertCmd", log.File, dest)
					// multiErr = errors.Join(multiErr, errFileisDir, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %w, %w", multiErr, errFileisDir, err)
				}
			}

			// validate flags
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				logger.Error(err, "Failed to parse flag", log.Command, "convertCmd", log.Flag, "force")
				// multiErr = errors.Join(multiErr, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			av, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				logger.Error(err, "Failed to parse flag", log.Command, "convertCmd", log.Flag, "aerospike-version")
				// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
				multiErr = fmt.Errorf("%w, %w, %w", multiErr, errInvalidAerospikeVersion, err)
			}

			if !force {
				supported, err := asconfig.IsSupportedVersion(av)
				if err != nil {
					logger.Error(err, "IsSupportedVersion returned an error", log.Command, "convertCmd", log.Flag, "aerospike-version", log.Value, av)
					// multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %w, %w", multiErr, errInvalidAerospikeVersion, err)
				}

				// TODO include valid versions in the error message
				// asconfig lib needs a getSupportedVersions func
				if !supported {
					logger.Error(errUnsupportedAerospikeVersion, "Unsupported aerospike version", log.Command, "convertCmd", log.Flag, "aerospike-version", log.Value, av)
					// multiErr = errors.Join(multiErr, errUnsupportedAerospikeVersion) TODO use this in go 1.20
					multiErr = fmt.Errorf("%w, %w, %w", multiErr, errUnsupportedAerospikeVersion, err)
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
