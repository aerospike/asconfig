package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aerospike/asconfig/schema"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listSchemasCmd)
}

var listSchemasCmd = newListSchemasCmd()

func newListSchemasCmd() *cobra.Command {
	res := &cobra.Command{
		Use:     "list-schemas",
		Short:   "List available Aerospike schema versions.",
		Long:    `List all available Aerospike schema versions that can be used with the schema-diff command.`,
		Example: `  asconfig list-schemas`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running list-schemas command")

			// Load schema map
			schemaMap, err := schema.NewSchemaMap()
			if err != nil {
				return fmt.Errorf("failed to load schema map: %w", err)
			}

			// Get all version keys (exclude README)
			var versions []string
			for version := range schemaMap {
				if !strings.Contains(strings.ToLower(version), "readme") {
					versions = append(versions, version)
				}
			}

			// Sort versions
			sort.Strings(versions)

			// Get output format
			format, _ := cmd.Flags().GetString("format")

			// Display versions
			if format == "table" {
				cmd.Printf("Available Aerospike Schema Versions:\n")
				cmd.Printf("====================================\n")
				for i, version := range versions {
					cmd.Printf("%2d. %s\n", i+1, version)
				}
				cmd.Printf("\nTotal: %d versions\n", len(versions))
			} else {
				// Simple format (default)
				cmd.Println(strings.Join(versions, "\n"))
			}

			return nil
		},
	}

	res.Flags().StringP("format", "f", "simple", "Output format: 'simple' (space-separated) or 'table' (numbered list)")

	return res
}
