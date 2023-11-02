package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/asconfig/asconf"
	"github.com/spf13/cobra"
)

const (
	convertArgMax         = 1
	defaultOutputFileName = "config"
)

var (
	errTooManyArguments            = fmt.Errorf("expected a maximum of %d arguments", convertArgMax)
	errFileNotExist                = fmt.Errorf("file does not exist")
	errMissingAerospikeVersion     = fmt.Errorf("missing required flag '--aerospike-version'")
	errInvalidAerospikeVersion     = fmt.Errorf("aerospike version must be in the form <a>.<b>.<c>")
	errUnsupportedAerospikeVersion = fmt.Errorf("aerospike version unsupported")
	errInvalidOutput               = fmt.Errorf("invalid output flag")
	errInvalidFormat               = fmt.Errorf("invalid format flag")
)

func init() {
	rootCmd.AddCommand(convertCmd)
}

var convertCmd = newConvertCmd()

func newConvertCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "convert [flags] <path/to/config.yaml>",
		Short: "Convert between yaml and Aerospike config format.",
		Long: `Convert is used to convert between yaml and aerospike configuration
				files. Input files are converted to their opposite format, yaml -> conf, conf -> yaml.
				The convert command validates the configuration file for compatibility with the Aerospike
				version passed to the --aerospike-version option, unless the --force option is used.
				Specifying the server version that will use the aerospike.conf is required.
				Usage examples...
				convert local file "aerospike.yaml" to aerospike config format for version 6.2.0 and
				write it to local file "aerospike.conf."
				Ex: asconfig convert --aerospike-version "6.2.0" aerospike.yaml --output aerospike.conf
				Short form flags and source file only conversions are also supported.
				In this case, -a is the server version and using only a source file means
				the result will be written to stdout.
				Ex: asconfig convert -a "6.2.0" aerospike.yaml
				Normally the file format is inferred from file extensions ".yaml" ".conf" etc.
				Source format can be forced with the --format flag.
				Ex: asconfig convert -a "6.2.0" --format yaml example_file
				If a file path is not provided, asconfig reads the file contents from stdin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running convert command")

			// read stdin by default
			var srcPath string
			if len(args) == 0 {
				srcPath = os.Stdin.Name()
			} else {
				srcPath = args[0]
			}

			version, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag aerospike-version value=%s", version)

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag force value=%t", force)

			srcFormat, err := getConfFileFormat(srcPath, cmd)
			if err != nil {
				return err
			}

			logger.Debugf("Processing flag format value=%v", srcFormat)

			fdata, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			var outFmt asconf.Format
			switch srcFormat {
			case asconf.AsConfig:
				outFmt = asconf.YAML
			case asconf.YAML:
				outFmt = asconf.AsConfig
			default:
				return fmt.Errorf("%w: %s", errInvalidFormat, srcFormat)
			}

			conf, err := asconf.NewAsconf(
				fdata,
				srcFormat,
				outFmt,
				version,
				logger,
				managementLibLogger,
			)

			if err != nil {
				return err
			}

			if !force {
				err = conf.Validate()
				if err != nil {
					return err
				}
			}

			out, err := conf.MarshalText()
			if err != nil {
				return err
			}

			outputPath, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}

			if stat, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) && stat.IsDir() {
				// output path is a directory so write a new file to it
				outFileName := filepath.Base(srcPath)
				if srcPath == os.Stdin.Name() {
					outFileName = defaultOutputFileName
				}

				outFileName = strings.TrimSuffix(outFileName, filepath.Ext(outFileName))

				outputPath = filepath.Join(outputPath, outFileName)
				if outFmt == asconf.YAML {
					outputPath += ".yaml"
				} else if outFmt == asconf.AsConfig {
					outputPath += ".conf"
				} else {
					return fmt.Errorf("output format unrecognized %w", errInvalidFormat)
				}
			}

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
			_, err = outFile.Write([]byte(out))
			return err

		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > convertArgMax {
				return errTooManyArguments
			}

			if len(args) > 0 {
				source := args[0]
				if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
					return errors.Join(errFileNotExist, err)
				}
			}

			// validate flags
			_, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			if !force {
				cmd.MarkFlagRequired("aerospike-version")
			}

			av, err := cmd.Flags().GetString("aerospike-version")
			if err != nil {
				return err
			}

			if !force {
				if av == "" {
					return errors.Join(errMissingAerospikeVersion, err)
				}

				supported, err := asconfig.IsSupportedVersion(av)
				if err != nil {
					return errors.Join(errInvalidAerospikeVersion, err)
				}

				// TODO include valid versions in the error message
				// asconfig lib needs a getSupportedVersions func
				if !supported {
					return errUnsupportedAerospikeVersion
				}
			}

			return nil
		},
	}

	// flags and configuration settings
	// aerospike-version is marked required in this cmd's PreRun if the --force flag is not provided
	res.Flags().StringP("aerospike-version", "a", "", "Aerospike server version to validate the configuration file for. Ex: 6.2.0.\nThe first 3 digits of the Aerospike version number are required.\nThis option is required unless --force is used")
	res.Flags().BoolP("force", "f", false, "Override checks for supported server version and config validation")
	res.Flags().StringP("output", "o", os.Stdout.Name(), "File path to write output to")

	res.Version = VERSION

	return res
}
