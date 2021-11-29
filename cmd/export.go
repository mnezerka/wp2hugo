package cmd

import (
	"wp2hugo/wordpress"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export file1 [file2, ...]",
	Short: "Parse wp xml files and create hugo content from it",
	Long:  ``,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		wp := wordpress.NewWpExport(log)

		for i := 0; i < len(args); i++ {
			wp.ReadWpExport(args[i])
		}

		err := wp.Export()

		return err
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
