package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aerospike/asconfig/log"

	"github.com/aerospike/asconfig/schema"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Replaced at compile time
var (
	VERSION = "development"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = newRootCmd()

var (
	errInvalidLogLevel = fmt.Errorf("Invalid log-level flag")
)

// newRootCmd is the root command constructor. It is useful for producing copies of rootCmd for testing.
func newRootCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "asconfig",
		Short: "Manage Aerospike configuration",
		Long:  "Asconfig is used to manage Aerospike configuration.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var multiErr error

			lvl, err := cmd.Flags().GetString("log-level")
			if err != nil {
				multiErr = fmt.Errorf("%w, %w", multiErr, err)
			}

			lvlCode, err := logrus.ParseLevel(lvl)
			if err != nil {
				multiErr = errors.Join(multiErr, errInvalidLogLevel, err)
			}

			logger.SetLevel(lvlCode)

			return multiErr
		},
	}

	res.Version = VERSION
	res.SetVersionTemplate("asconfig\n{{printf \"Version %s\" .Version}}\n")

	logLevelUsage := fmt.Sprintf("Set the logging detail level. Valid levels are: %v", log.GetLogLevels())
	res.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	res.PersistentFlags().BoolP("version", "V", false, "Version for asconfig.")
	res.PersistentFlags().BoolP("help", "u", false, "Display help information")

	res.SilenceErrors = true
	res.SilenceUsage = true

	res.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		logger.Error(err)
		cmd.Println(cmd.UsageString())
		return errors.Join(err, ErrSilent)
	})

	return res
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
