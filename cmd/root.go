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
	VERSION     = "development" // Replaced at compile time
	cfFileFlags = flags.NewConfFileFlags()
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

			cfgFiles, err := initConfigFiles(cmd)
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

			for _, cfgFile := range cfgFiles {
				logger.Infof("Using config file: %s", cfgFile)
			}

			if multiErr == nil {
				warnIgnoredConfigFileOptions(cmd)
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
		// Don't log help requests as errors
		if strings.Contains(err.Error(), "help requested") {
			cmd.Println(cmd.UsageString())
			return errors.Join(err, ErrSilent)
		}

		logger.Error(err)
		cmd.Println(cmd.UsageString())

		return errors.Join(err, ErrSilent)
	})

	return cmd
}

func readsConfigFiles(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations[configFilesAnnotation]
	return ok
}

func initConfigFiles(cmd *cobra.Command) ([]string, error) {
	files := cfFileFlags.Files()

	if !readsConfigFiles(cmd) {
		return nil, checkConfigFileOptions(files)
	}

	return config.InitConfigFiles(files, cfFileFlags.Instance, cmd.Flags())
}

func checkConfigFileOptions(files config.Files) error {
	switch {
	case files.None && files.Only != "":
		return errNoAndOnlyConfigFile
	case files.File != "" && files.Only != "":
		return errOnlyAndConfigFile
	case files.File != "" && files.None:
		return errNoAndConfigFile
	}

	return nil
}

func warnIgnoredConfigFileOptions(cmd *cobra.Command) {
	if readsConfigFiles(cmd) {
		return
	}

	for _, name := range []string{"config-file", "only-config-file", "no-config-file", "instance"} {
		if cmd.Flags().Changed(name) {
			logger.Warnf("--%s is ignored: %s reads no config file", name, cmd.CommandPath())
		}
	}
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
	rootCmd.AddCommand(newListCmd())
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
