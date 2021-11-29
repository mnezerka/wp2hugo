package cmd

import (
	"wp2hugo/wordpress"

	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump file1 [file2, ...]",
	Short: "Parse wp xml files and dump consolidated data",
	Long:  ``,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		wp := wordpress.NewWpExport(log)

		for i := 0; i < len(args); i++ {
			wp.ReadWpExport(args[i])
		}

		wp.Dump()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}
