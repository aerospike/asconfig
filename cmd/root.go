package cmd

import (
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
	ArgMin = 1
	ArgMax = 2
)

var (
	errNotEnoughArguments      = fmt.Errorf("ERR: expected a minimum of %d arguments", ArgMin)
	errTooManyArguments        = fmt.Errorf("ERR: expected a maximum of %d arguments", ArgMax)
	errFileNotExist            = fmt.Errorf("ERR: file does not exist")
	errInvalidAerospikeVersion = fmt.Errorf("ERR: aerospike version must be in the form <a>.<b>.<c>.<d>")
)

// Replaced at compile time
var (
	VERSION = "0.0.1"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
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
			return fmt.Errorf("failed to load config map by lib: %v", err)
		}

		confFile := asConf.ToConfFile()

		// TODO asconf to yaml

		destPath := strings.TrimSuffix(srcPath, ".yaml") + ".conf"
		if len(args) > ArgMin {
			destPath = args[1]
			log.Info("processing argument", argNameKey, "destination", valueKey, destPath)
		}

		log.Info("writing converted data to", fileKey, destPath)
		return os.WriteFile(destPath, []byte(confFile), 0644)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var multiErr error

		// validate arguments
		if len(args) < ArgMin {
			errors.Join(multiErr, errNotEnoughArguments)
			return multiErr
		}

		if len(args) > ArgMax {
			errors.Join(multiErr, errTooManyArguments)
		}

		source := args[0]
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			errors.Join(multiErr, errFileNotExist, err)
		}

		if len(args) > ArgMin {
			dest := args[1]
			if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
				errors.Join(multiErr, errFileNotExist, err)
			}
		}

		// validate flags
		av, err := cmd.Flags().GetString("aerospike-version")
		if err != nil {
			errors.Join(multiErr, errInvalidAerospikeVersion, err)
		}

		supported, err := asconfig.IsSupportedVersion(av)
		if err != nil {
			errors.Join(multiErr, errInvalidAerospikeVersion, err) // TODO use an error for unsupported versions and list supported versions
		}

		if !supported {
			errors.Join(multiErr, errInvalidAerospikeVersion)
		}

		if multiErr != nil {
			errors.Join(multiErr, errInvalidAerospikeVersion)
		}

		return multiErr
	},
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
	// logging
	log = logrusr.New(logrus.New())

	// flags and configuration settings
	rootCmd.Flags().BoolP("help", "u", false, "Display help information")
	rootCmd.PersistentFlags().StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.2")
	rootCmd.MarkPersistentFlagRequired("aerospike-version")

	rootCmd.Version = VERSION
}
