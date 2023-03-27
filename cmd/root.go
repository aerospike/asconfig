package cmd

import (
	"aerospike/asconfig/log"
	"aerospike/asconfig/schema"
	"os"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
		Use:   "asconfig",
		Short: "Manage Aerospike configuration",
		Long:  "Asconfig is used to manage Aerospike configuration.",
	}

	res.Version = VERSION

	return res
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger.Error(err, "Execute failed", log.Command, "rootCmd")
		os.Exit(1)
	}
}

var logger logr.Logger

func init() {
	tmpLog := logrus.New()

	fmt := logrus.TextFormatter{}
	fmt.FullTimestamp = true

	tmpLog.SetFormatter(&fmt)
	logger = logrusr.New(tmpLog)

	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		panic(err)
	}

	asconfig.InitFromMap(logger, schemaMap)
}
