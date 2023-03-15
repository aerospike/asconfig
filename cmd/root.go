package cmd

import (
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
		log.Error(err, "execute failed", keyCmdName, "rootCmd")
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
