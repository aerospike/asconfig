package cmd

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/aerospike/asconfig/asconf"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// common flags
func getCommonFlags() *pflag.FlagSet {
	res := &pflag.FlagSet{}
	res.StringP("aerospike-version", "a", "", "Aerospike server version for the configuration file. Ex: 6.2.0.\nThe first 3 digits of the Aerospike version number are required.")

	return res
}

// getConfFileFormat guesses the format of an input config file
// based on file extension and the --format flag of the cobra command
// this function implements the defaults scheme for file formats in asconfig
// if the --format flag is defined use that, else if the path has an extension
// use that, else use the default value from --format
func getConfFileFormat(path string, cmd *cobra.Command) (asconf.Format, error) {
	ext := filepath.Ext(path)
	ext = strings.TrimPrefix(ext, ".")

	fmtStr, err := cmd.Flags().GetString("format")
	if err != nil {
		return asconf.Invalid, err
	}

	// if the user did not supply format, and
	// the input file has an extension, overwrite it with ext
	if !cmd.Flags().Changed("format") && ext != "" {
		fmtStr = ext
	}

	fmt, err := asconf.ParseFmtString(fmtStr)
	if err != nil {
		return asconf.Invalid, err
	}

	return fmt, nil
}

var SilentError = errors.New("SILENT")
