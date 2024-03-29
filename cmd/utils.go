package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aerospike/asconfig/conf"
	"github.com/aerospike/asconfig/conf/metadata"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	metaKeyAerospikeVersion = "aerospike-server-version"
	metaKeyAsconfigVersion  = "asconfig-version"
	metaKeyAsadmVersion     = "asadm-version"
)

type metaDataArgs struct {
	src              []byte
	aerospikeVersion string
	asconfigVersion  string
}

func genMetaDataText(src []byte, msg []byte, mdata map[string]string) ([]byte, error) {

	metaHeader := "# *** Aerospike Metadata Generated by Asconfig ***"

	err := metadata.Unmarshal(src, mdata)
	if err != nil {
		return nil, err
	}

	mtext, err := metadata.Marshal(mdata)
	if err != nil {
		return nil, err
	}

	metaFooter := "# *** End Aerospike Metadata ***"
	strMsg := string(msg)

	if len(msg) > 0 {
		strMsg = strMsg + "\n#\n"
	}

	mtext = []byte(fmt.Sprintf("%s\n%s%s%s\n\n", metaHeader, strMsg, mtext, metaFooter))

	return mtext, nil
}

func getMetaDataItemOptional(src []byte, key string) (string, error) {
	mdata := map[string]string{}
	err := metadata.Unmarshal(src, mdata)
	if err != nil {
		return "", err
	}

	val := mdata[key]

	return val, nil
}

func getMetaDataItem(src []byte, key string) (string, error) {
	val, err := getMetaDataItemOptional(src, key)
	if err != nil {
		return "", err
	}

	if val == "" {
		return "", fmt.Errorf("metadata does not contain %s", key)
	}

	return val, nil
}

// common flags
func getCommonFlags() *pflag.FlagSet {
	res := &pflag.FlagSet{}
	res.StringP("aerospike-version", "a", "", "Aerospike server version to validate the configuration file for. Ex: 6.2.0.\nThe first 3 digits of the Aerospike version number are required.\nThis option is required unless --force is used.")

	return res
}

// getConfFileFormat guesses the format of an input config file
// based on file extension and the --format flag of the cobra command
// this function implements the defaults scheme for file formats in asconfig
// if the --format flag is defined use that, else if the path has an extension
// use that, else use the default value from --format
func getConfFileFormat(path string, cmd *cobra.Command) (conf.Format, error) {
	ext := filepath.Ext(path)
	ext = strings.TrimPrefix(ext, ".")

	fmtStr, err := cmd.Flags().GetString("format")
	if err != nil {
		return conf.Invalid, err
	}

	logger.Debugf("Processing flag format value=%v", fmtStr)

	// if the user did not supply format, and
	// the input file has an extension, overwrite it with ext
	if !cmd.Flags().Changed("format") && ext != "" {
		fmtStr = ext
	}

	fmt, err := ParseFmtString(fmtStr)
	if err != nil {
		return conf.Invalid, err
	}

	return fmt, nil
}

var ErrSilent = errors.New("SILENT")

func ParseFmtString(in string) (f conf.Format, err error) {

	switch strings.ToLower(in) {
	case "yaml", "yml":
		f = conf.YAML
	case "asconfig", "conf", "asconf":
		f = conf.AsConfig
	default:
		f = conf.Invalid
		err = fmt.Errorf("%w: %s", conf.ErrInvalidFormat, in)
	}

	return
}
