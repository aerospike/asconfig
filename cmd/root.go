package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aerospike/asconfig/log"
	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"

	"github.com/aerospike/asconfig/schema"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = newRootCmd()

var (
	VERSION            = "development" // Replaced at compile time
	errInvalidLogLevel = fmt.Errorf("Invalid log-level flag")
	aerospikeFlags     = flags.NewDefaultAerospikeFlags()
	cfFileFlags        = flags.NewConfFileFlags()
)

// newRootCmd is the root command constructor. It is useful for producing copies of rootCmd for testing.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "asconfig",
		Short: "Manage Aerospike configuration",
		Long:  "Asconfig is used to manage Aerospike configuration.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
				multiErr = errors.Join(multiErr, errInvalidLogLevel, err)
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
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	rootCmd.PersistentFlags().AddFlagSet(cfFileFlags.NewFlagSet(flags.DefaultWrapHelpString))
	flags.SetupRoot(rootCmd, "Aerospike Config", VERSION)

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		logger.Error(err)
		cmd.Println(cmd.UsageString())
		return errors.Join(err, ErrSilent)
	})

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		if !errors.Is(err, ErrSilent) {
			// handle wrapped errors
			errs := strings.Split(err.Error(), "\n")

			for _, err := range errs {
				logger.Error(err)
			}
		}
		os.Exit(1)
	}
}

var logger *logrus.Logger
var mgmtLibLogger logr.Logger

func init() {
	logger = logrus.New()

	fmt := logrus.TextFormatter{}
	fmt.FullTimestamp = true

	logger.SetFormatter(&fmt)

	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		panic(err)
	}

	mgmtLibLogger = logrusr.New(logger)
	asconfig.InitFromMap(mgmtLibLogger, schemaMap)
}
