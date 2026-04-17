package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"

	"github.com/aerospike/asconfig/conf/serveryaml"
)

const flagServerYAML = "server-yaml"

const flagServerYAMLDescription = "Treat YAML input/output as the server-native (experimental) YAML format introduced in Aerospike 8.1.0. " +
	"When the command reads YAML, the input is interpreted as server-native YAML and validated against the experimental schema. " +
	"When the command writes YAML, the output is emitted in server-native form. " +
	"Without this flag YAML is treated as legacy asconfig YAML."

var (
	errServerYAMLRequiresVersion = errors.New(
		"--server-yaml requires an aerospike-server-version from the --aerospike-version flag or file metadata",
	)
	errServerYAMLUnsupportedVersion = fmt.Errorf(
		"--server-yaml requires aerospike-server-version >= %s",
		serveryaml.MinSupportedVersion,
	)
	errServerYAMLSchemaRejection = errors.New("server-native yaml failed schema validation")
)

// isServerYAMLFlagSet returns whether --server-yaml was supplied on cmd.
func isServerYAMLFlagSet(cmd *cobra.Command) (bool, error) {
	if cmd == nil {
		return false, nil
	}

	flag := cmd.Flags().Lookup(flagServerYAML)
	if flag == nil {
		return false, nil
	}

	return cmd.Flags().GetBool(flagServerYAML)
}

// prepareYAMLForParse validates a YAML document against the experimental
// schema for the supplied aerospike-server-version and translates it into the
// legacy asconfig YAML shape suitable for aerospike-management-lib. Callers
// should invoke this whenever --server-yaml is honored and a concrete version
// is available (validate, convert, diff server).
//
// The flag is context-sensitive and applies only to the YAML side of a given
// operation, so this helper is a no-op when the flag is absent or when the
// source is not YAML. For conversions like `convert conf -> yaml`, the
// --server-yaml flag is honored on the output side via maybeEmitNativeYAML.
func prepareYAMLForParse(
	cmd *cobra.Command,
	srcFormat asConf.Format,
	version string,
	cfgData []byte,
) ([]byte, error) {
	set, err := isServerYAMLFlagSet(cmd)
	if err != nil {
		return nil, err
	}

	if !set || srcFormat != asConf.YAML {
		return cfgData, nil
	}

	if version == "" {
		return nil, errServerYAMLRequiresVersion
	}

	supported, err := serveryaml.IsSupportedVersion(version)
	if err != nil {
		return nil, err
	}

	if !supported {
		return nil, fmt.Errorf("%w: got %s", errServerYAMLUnsupportedVersion, version)
	}

	verrs, err := serveryaml.Validate(cfgData, version)
	if err != nil {
		return nil, err
	}

	if len(verrs) > 0 {
		return nil, formatServerYAMLValidationErrors(verrs)
	}

	return serveryaml.ToLegacy(cfgData)
}

// translateNativeYAMLForDiff translates server-native YAML to the legacy
// asconfig shape without performing any schema validation. `diff files` is
// intentionally version-agnostic and schema-free, so --server-yaml only acts
// as a signal that the input is in the server-native shape. When the source
// is not YAML the flag does not apply to that file and the bytes are returned
// unchanged.
func translateNativeYAMLForDiff(
	cmd *cobra.Command,
	srcFormat asConf.Format,
	cfgData []byte,
) ([]byte, error) {
	set, err := isServerYAMLFlagSet(cmd)
	if err != nil {
		return nil, err
	}

	if !set || srcFormat != asConf.YAML {
		return cfgData, nil
	}

	return serveryaml.ToLegacy(cfgData)
}

// maybeEmitNativeYAML translates legacy asconfig YAML output into the
// server-native (experimental) shape when --server-yaml is enabled. The
// supplied version must satisfy serveryaml.MinSupportedVersion.
//
// The flag applies only to the YAML side of the operation, so this helper is
// a no-op when the flag is absent or when the output format is not YAML.
func maybeEmitNativeYAML(
	cmd *cobra.Command,
	outFormat asConf.Format,
	version string,
	cfgData []byte,
) ([]byte, error) {
	set, err := isServerYAMLFlagSet(cmd)
	if err != nil {
		return nil, err
	}

	if !set || outFormat != asConf.YAML {
		return cfgData, nil
	}

	if version == "" {
		return nil, errServerYAMLRequiresVersion
	}

	supported, err := serveryaml.IsSupportedVersion(version)
	if err != nil {
		return nil, err
	}

	if !supported {
		return nil, fmt.Errorf("%w: got %s", errServerYAMLUnsupportedVersion, version)
	}

	return serveryaml.FromLegacy(cfgData)
}

// formatServerYAMLValidationErrors renders validation failures from
// serveryaml.Validate in the same shape as conf.ValidationErrors so users see
// a consistent experience regardless of which schema rejected the file.
func formatServerYAMLValidationErrors(verrs []serveryaml.ValidationError) error {
	errorsByContext := map[string][]serveryaml.ValidationError{}
	for _, verr := range verrs {
		errorsByContext[verr.Context] = append(errorsByContext[verr.Context], verr)
	}

	contexts := make([]string, 0, len(errorsByContext))
	for ctx := range errorsByContext {
		contexts = append(contexts, ctx)
	}

	sort.Strings(contexts)

	var buf strings.Builder

	for _, ctx := range contexts {
		fmt.Fprintf(&buf, "context: %s\n", ctx)
		for _, verr := range errorsByContext[ctx] {
			if verr.ErrType == "number_one_of" {
				continue
			}

			fmt.Fprintf(&buf, "\t- %s\n", verr.Error())
		}
	}

	return fmt.Errorf("%s\n%w", buf.String(), errServerYAMLSchemaRejection)
}
