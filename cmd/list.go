package cmd

import (
	"fmt"
	"sort"
	"strings"

	lib "github.com/aerospike/aerospike-management-lib"
	"github.com/aerospike/asconfig/schema"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "list",
		Short: "List the available Aerospike server versions.",
		Long:  `List is used to list the available Aerospike server versions.`,
		Example: `  asconfig list versions
  asconfig list versions --verbose`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runListCommand(cmd)
		},
	}

	res.Version = VERSION
	res.AddCommand(newListVersionsCmd())
	return res
}

func newListVersionsCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "versions",
		Short: "List available Aerospike server versions.",
		Long:  `List all available Aerospike server versions that can be used with the diff versions command.`,
		Example: `  asconfig list versions
  asconfig list versions --verbose`,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			// Sort versions using semantic version comparison
			sort.Slice(versions, func(i, j int) bool {
				cmp, compErr := lib.CompareVersions(versions[i], versions[j])
				if compErr != nil {
					// Fall back to lexical order if comparison fails
					logger.Warnf("Falling back to lexical version sort: %v", compErr)
					return versions[i] < versions[j]
				}
				return cmp < 0
			})

			// Get output format
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Display versions
			if verbose {
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

	res.Flags().BoolP("verbose", "v", false, "Display output in verbose format with numbering")
	res.Version = VERSION

	return res
}

func runListCommand(cmd *cobra.Command) error {
	// Show help when no subcommand is provided
	return cmd.Help()
}
