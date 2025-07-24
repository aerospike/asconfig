package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aerospike/asconfig/schema"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listVersionsCmd)
}

var listVersionsCmd = newListVersionsCmd()

func newListVersionsCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "list-versions",
		Short: "List available Aerospike server versions.",
		Long:  `List all available Aerospike server versions that can be used with the diff-versions command.`,
		Example: `  asconfig list-versions
  asconfig list-versions --table`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running list-versions command")

			// Load schema map
			schemaMap, err := schema.NewSchemaMap()
			if err != nil {
				return fmt.Errorf("failed to load schema map: %w", err)
			}

			var versions []string
			for version := range schemaMap {
				versions = append(versions, version)
			}

			// Sort versions
			sort.Strings(versions)

			// Get output format
			table, _ := cmd.Flags().GetBool("table")

			// Display versions
			if table {
				cmd.Printf("Available Aerospike Server Versions:\n")
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

	res.Flags().BoolP("table", "t", false, "Display output in table format with numbering")

	return res
}
