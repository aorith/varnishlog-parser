package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var Version string = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "varnishlog-parser",
	Short: "A varnishlog parser library and web user interface",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
