package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v6"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/asconfig/asconf"
	"github.com/spf13/cobra"
)

const (
	generateArgMax = 1
)

func init() {
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = newGenerateCmd()

func newGenerateCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "generate [flags]",
		Short: "Generate an configuration file from a running Aerospike node.",
		Long:  `TODO`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running generate command")

			// write stdout by default
			var dstPath string
			if len(args) == 0 {
				dstPath = os.Stdout.Name()
			} else {
				dstPath = args[0]
			}

			outFormat := asconf.AsConfig

			logger.Debugf("Processing flag format value=%v", outFormat)

			logger.Debugf("Generating config from Aerospike node")
			asPolicy := as.NewClientPolicy()
			asPolicy.User = "admin"
			asPolicy.Password = "admin"
			asinfo := info.NewAsInfo(managementLibLogger, as.NewHost("172.17.0.5", 3000), asPolicy)

			f, err := os.Create("profile.prof")
			if err != nil {
				logger.Fatal(err)
			}

			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()

			generatedConf, err := asconfig.GenerateConf(managementLibLogger, asinfo, true)

			if err != nil {
				return errors.Join(fmt.Errorf("unable to generate config file"), err)
			}

			conf, err := asconfig.NewMapAsConfig(managementLibLogger, generatedConf.Version, generatedConf.Conf)

			if err != nil {
				return errors.Join(fmt.Errorf("unable to parse the generated conf file"), err)
			}

			confFile := conf.ToConfFile()

			if stat, err := os.Stat(dstPath); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
				// output path is a directory so write a new file to it
				outFileName := filepath.Base(dstPath)
				if dstPath == os.Stdin.Name() {
					outFileName = defaultOutputFileName
				}

				outFileName = strings.TrimSuffix(outFileName, filepath.Ext(outFileName))

				dstPath = filepath.Join(dstPath, outFileName)
				if outFormat == asconf.YAML {
					dstPath += ".yaml"
				} else if outFormat == asconf.AsConfig {
					dstPath += ".conf"
				} else {
					return fmt.Errorf("output format unrecognized %w", errInvalidFormat)
				}
			}

			var outFile *os.File
			if dstPath == os.Stdout.Name() {
				outFile = os.Stdout
			} else {
				outFile, err = os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					return err
				}

				defer outFile.Close()
			}

			logger.Debugf("Writing converted data to: %s", dstPath)
			_, err = outFile.Write([]byte(confFile))
			return err

		},
		PreRunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	// flags and configuration settings
	// aerospike-version is marked required in this cmd's PreRun if the --force flag is not provided
	// commonFlags := getCommonFlags()
	// res.Flags().AddFlagSet(commonFlags)
	// res.Flags().BoolP("force", "f", false, "Override checks for supported server version and config validation")

	res.Version = VERSION

	return res
}
