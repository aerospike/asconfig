package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/asconfig/conf"
	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/spf13/cobra"
)

var generateArgMax = 1

func init() {
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = newGenerateCmd()

func newGenerateCmd() *cobra.Command {
	asCommonFlags := flags.NewDefaultAerospikeFlags()
	disclaimer := []byte(`#
# This configuration file is generated by asconfig, this feature is currently in beta. 
# We appreciate your feedback on any issues encountered. These can be reported 
# to our support team or via GitHub. Please ensure to verify the configuration 
# file before use. Current limitations include the inability to generate the 
# following contexts and parameters: logging.syslog, mod-lua, service.user, 
# service.group. Please note that this configuration file may not be compatible 
# with all versions of Aerospike or the Community Edition.`)
	res := &cobra.Command{
		Use:   "generate [flags]",
		Short: "BETA: Generate a configuration file from a running Aerospike node.",
		Long:  `BETA: Generate a configuration file from a running Aerospike node. This can be useful if you have changed the configuration of a node dynamically (e.g. xdr) and would like to persist the changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running generate command")

			outputPath, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}

			outFormat, err := getConfFileFormat(outputPath, cmd)
			if err != nil {
				return err
			}

			logger.Debugf("Generating config from Aerospike node")

			asCommonConfig := aerospikeFlags.NewAerospikeConfig()

			asPolicy, err := asCommonConfig.NewClientPolicy()
			if err != nil {
				return errors.Join(fmt.Errorf("unable to create client policy"), err)
			}

			logger.Infof("Retrieving Aerospike configuration from node %s", &asCommonFlags.Seeds)

			asHosts := asCommonConfig.NewHosts()
			asinfo := info.NewAsInfo(mgmtLibLogger, asHosts[0], asPolicy)

			generatedConf, err := asconfig.GenerateConf(mgmtLibLogger, asinfo, true)
			if err != nil {
				return errors.Join(fmt.Errorf("unable to generate config file"), err)
			}

			asconfig, err := asconfig.NewMapAsConfig(mgmtLibLogger, generatedConf.Conf)
			if err != nil {
				return errors.Join(fmt.Errorf("unable to parse the generated conf file"), err)
			}

			marshaller := conf.NewConfigMarshaller(asconfig, outFormat)

			fdata, err := marshaller.MarshalText()
			if err != nil {
				return errors.Join(fmt.Errorf("unable to marshal the generated conf file"), err)
			}

			mdata := map[string]string{
				metaKeyAerospikeVersion: generatedConf.Version,
				metaKeyAsconfigVersion:  VERSION,
			}
			// prepend metadata to the config output
			mtext, err := genMetaDataText(fdata, disclaimer, mdata)
			if err != nil {
				return err
			}

			fdata = append(mtext, fdata...)

			var outFile *os.File
			if outputPath == os.Stdout.Name() {
				outFile = os.Stdout
			} else {
				outFile, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					return err
				}

				defer outFile.Close()
			}

			logger.Debugf("Writing converted data to: %s", outputPath)
			_, err = outFile.Write(fdata)

			logger.Warning(
				"Community Edition is not supported. Generated static configuration does not save logging.syslog, mod-lua, service.user and service.group",
			)
			logger.Warning(
				"This feature is currently in beta. Use at your own risk and please report any issue to support.",
			)

			return err
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > generateArgMax {
				return errTooManyArguments
			}

			// validate flags
			_, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}

			formatString, err := cmd.Flags().GetString("format")
			if err != nil {
				return errors.Join(errMissingFormat, err)
			}

			_, err = ParseFmtString(formatString)
			if err != nil && formatString != "" {
				return errors.Join(errInvalidFormat, err)
			}

			return nil
		},
	}

	res.Version = VERSION
	asFlagSet := aerospikeFlags.NewFlagSet(flags.DefaultWrapHelpString)
	res.Flags().AddFlagSet(asFlagSet)
	config.BindPFlags(asFlagSet, "cluster")

	res.Flags().StringP("output", "o", os.Stdout.Name(), flags.DefaultWrapHelpString("File path to write output to"))
	res.Flags().StringP("format", "F", "conf", flags.DefaultWrapHelpString("The format of the destination file(s). Valid options are: yaml, yml, and conf."))

	return res
}
