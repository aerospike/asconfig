package cmd

import (
	"aerospike/asconfig/schema"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
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
	errInvalidAerospikeVersion     = fmt.Errorf("aerospike version must be in the form <a>.<b>.<c>.<d>")
	errUnsupportedAerospikeVersion = fmt.Errorf("aerospike version unsupported")
	errConfigValidation            = fmt.Errorf("error while validating aerospike config")
)

// Replaced at compile time
var (
	VERSION = "0.0.1"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = newRootCmd()

// newRootCmd is the root command constructor. It is useful for producing copies of rootCmd for tesing.
func newRootCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "asconfig <path/to/config.yaml> [<path/to/aerospike.conf>]",
		Short: "Convert yaml to aerospike config format.",
		Long: `Asconfig is used to convert between yaml and aerospike configuration
				files. In the future, this tool may be able to convert from asconf back to yaml.
				Specifying the server version that will use the aerospike.conf is required.
				Usage examples...
				convert local file "aerospike.yaml" to aerospike config format for version 6.2.0.2 and
				write it to local file "aerospike.conf."
				EX: asconfig --aerospike-version "6.2.0.2" aerospike.yaml aerospike.conf
				Short form flags and source file only conversions are also supported.
				In this case, -a is the server version and using only a source file means
				the result will be written as <path/to/config>.conf
				EX: asconfig -a "6.2.0.2 aerospike.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("running command", cmdNameKey, "rootCmd")

			srcPath := args[0]
			log.Info("processing argument", argNameKey, "source", valueKey, srcPath)

			version, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}
			log.Info("processing flag", flagNameKey, "aerospike-version", valueKey, version)

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}
			log.Info("processing flag", flagNameKey, "force", valueKey, force)

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			var data map[string]any
			err = yaml.Unmarshal(fdata, &data)
			if err != nil {
				return err
			}

			asConf, err := asconfig.NewMapAsConfig(log, version, data)
			if err != nil {
				return fmt.Errorf("failed to load config map: %v", err)
			}

			if !force {
				valid, validationErrors, err := asConf.IsValid(log, version)
				if !valid {
					log.Error(errConfigValidation, "valid is false", fileKey, srcPath)
				}
				if len(validationErrors) > 0 {
					for _, e := range validationErrors {
						errorKeysAndValues, err := structToKeysAndValues(*e)
						if err != nil {
							return err
						}

						keysAndValues := append([]any{fileKey, srcPath}, errorKeysAndValues...)
						log.Error(errConfigValidation, "validationErrors", keysAndValues...)
					}
				}
				if err != nil {
					return err
				}
			}

			confFile := asConf.ToConfFile()

			// TODO asconf to yaml

			destPath := strings.TrimSuffix(srcPath, ".yaml") + ".conf"
			if len(args) > argMin {
				destPath = args[1]
				log.Info("processing argument", argNameKey, "destination", valueKey, destPath)
			}

			log.Info("writing converted data to", fileKey, destPath)
			return os.WriteFile(destPath, []byte(confFile), 0644)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var multiErr error

			// validate arguments
			if len(args) < argMin {
				log.Error(errNotEnoughArguments, "too few arguments", cmdNameKey, "rootCmd", countKey, len(args), expectedKey, argMin)
				multiErr = errors.Join(multiErr, errNotEnoughArguments)
				return multiErr
			}

			if len(args) > argMax {
				log.Error(errTooManyArguments, "too many arguments", cmdNameKey, "rootCmd", countKey, len(args), expectedKey, argMax)
				multiErr = errors.Join(multiErr, errTooManyArguments)
			}

			source := args[0]
			if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
				log.Error(errFileNotExist, "source file does not exist", cmdNameKey, "rootCmd", fileKey, source)
				multiErr = errors.Join(multiErr, errFileNotExist, err)
			}

			if len(args) > argMin {
				dest := args[1]
				if stat, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
					log.Error(errFileisDir, "file to write to is a directory", cmdNameKey, "rootCmd", fileKey, dest)
					multiErr = errors.Join(multiErr, errFileisDir, err)
				}
			}

			// validate flags
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				log.Error(err, "failed to parse flag", cmdNameKey, "rootCmd", flagNameKey, "force")
				multiErr = errors.Join(multiErr, err)
			}

			av, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				log.Error(err, "failed to parse flag", cmdNameKey, "rootCmd", flagNameKey, "aerospike-version")
				multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err)
			}

			if !force {
				supported, err := asconfig.IsSupportedVersion(av)
				if err != nil {
					log.Error(err, "IsSupportedVersion returned an error", cmdNameKey, "rootCmd", flagNameKey, "aerospike-version", valueKey, av)
					multiErr = errors.Join(multiErr, errInvalidAerospikeVersion, err)
				}

				// TODO include valide versions in the error message
				// asconfig lib needs a getSupportedVersions func
				if !supported {
					log.Error(errUnsupportedAerospikeVersion, "unsupported aerospike version", cmdNameKey, "rootCmd", flagNameKey, "aerospike-version", valueKey, av)
					multiErr = errors.Join(multiErr, errUnsupportedAerospikeVersion)
				}
			}

			return multiErr
		},
	}

	// flags and configuration settings
	res.Flags().BoolP("help", "V", false, "Display help information")
	res.PersistentFlags().StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.2")
	res.MarkPersistentFlagRequired("aerospike-version")
	res.PersistentFlags().BoolP("force", "f", false, "Override checks for supported server version and config validation")

	res.Version = VERSION

	return res
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Error(err, "execute failed", cmdNameKey, "rootCmd")
		os.Exit(1)
	}
}

var log logr.Logger

func init() {
	log = logrusr.New(logrus.New())

	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		panic(err)
	}

	asconfig.InitFromMap(log, schemaMap)
}
