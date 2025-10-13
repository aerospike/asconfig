package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/aerospike/asconfig/log"
	"github.com/aerospike/asconfig/schema"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = NewRootCmd()

var (
	VERSION            = "development" // Replaced at compile time
	ErrInvalidLogLevel = errors.New("invalid log-level flag")
	aerospikeFlags     = flags.NewDefaultAerospikeFlags()
	cfFileFlags        = flags.NewConfFileFlags()
)

// NewRootCmd creates and returns the root cobra command.
// It is useful for producing copies of rootCmd for testing.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asconfig",
		Short: "Manage Aerospike configuration",
		Long:  "Asconfig is used to manage Aerospike configuration.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			var multiErr error

			cfgFile, err := config.InitConfig(cfFileFlags.File, cfFileFlags.Instance, cmd.Flags())
			if err != nil {
				multiErr = errors.Join(multiErr, err)
			}

			lvl, err := cmd.Flags().GetString("log-level")
			if err != nil {
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			lvlCode, err := logrus.ParseLevel(lvl)
			if err != nil {
				multiErr = errors.Join(multiErr, ErrInvalidLogLevel, err)
			}

			logger.SetLevel(lvlCode)

			if cfgFile != "" {
				logger.Infof("Using config file: %s", cfgFile)
			}

			return multiErr
		},
	}

	// TODO: log levels should be generic and not tied to logrus or golang.
	logLevelUsage := fmt.Sprintf("Set the logging detail level. Valid levels are: %v", log.GetLogLevels())
	cmd.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	cmd.PersistentFlags().AddFlagSet(cfFileFlags.NewFlagSet(flags.DefaultWrapHelpString))
	flags.SetupRoot(cmd, "Aerospike Config", VERSION)

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		logger.Error(err)
		cmd.Println(cmd.UsageString())

		return errors.Join(err, ErrSilent)
	})

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	// Initialize global loggers and schema
	if err := InitializeGlobals(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		return err
	}

	// Register subcommands
	rootCmd.AddCommand(newConvertCmd())
	rootCmd.AddCommand(newDiffCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newValidateCmd())

	err := rootCmd.Execute()
	if err != nil {
		if !errors.Is(err, ErrSilent) {
			// handle wrapped errors
			errs := strings.Split(err.Error(), "\n")

			for _, err := range errs {
				logger.Error(err)
			}
		}

		return err
	}

	return nil
}

var logger *logrus.Logger
var mgmtLibLogger logr.Logger

// InitializeGlobals initializes global loggers and schema.
func InitializeGlobals() error {
	logger = logrus.New()

	formatter := logrus.TextFormatter{}
	formatter.FullTimestamp = true

	logger.SetFormatter(&formatter)

	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		return err
	}

	mgmtLibLogger = logrusr.New(logger)
	asconfig.InitFromMap(mgmtLibLogger, schemaMap)

	return nil
}

// InitializeGlobalsForTesting initializes globals for testing, panicking on error.
// This is a test helper function.
func InitializeGlobalsForTesting() {
	if err := InitializeGlobals(); err != nil {
		panic("Failed to initialize globals for testing: " + err.Error())
	}
}
