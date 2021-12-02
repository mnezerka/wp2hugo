package cmd

import (
	"wp2hugo/wordpress"

	"github.com/spf13/cobra"
)

var (
	config_no_downloads bool
	config_no_comments  bool
	config_output_dir   string
)

var exportCmd = &cobra.Command{
	Use:   "export file1 [file2, ...]",
	Short: "Parse wp xml files and create hugo content from it",
	Long:  ``,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		wp := wordpress.NewWpExport(log)
		wp.ConfigNoDownloads = config_no_downloads
		wp.ConfigNoComments = config_no_comments
		wp.ConfigOutputDir = config_output_dir

		for i := 0; i < len(args); i++ {
			wp.ReadWpExport(args[i])
		}

		err := wp.Export()

		return err
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().BoolVarP(&config_no_downloads, "no-downloads", "d", false, "Do not download any media from remote server")
	exportCmd.Flags().BoolVarP(&config_no_comments, "no-comments", "c", false, "Do not typeset any comments")
	exportCmd.Flags().StringVarP(&config_output_dir, "output-dir", "o", "build", "Output directory")
}
