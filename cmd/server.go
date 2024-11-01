package cmd

import (
	"github.com/aorith/varnishlog-parser/pkg/server"
	"github.com/spf13/cobra"
)

var (
	bind string
	port int
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the http server",
	Long: `Start the http server, for example:

server --bind 0.0.0.0 --port 8080
`,
	Run: func(cmd *cobra.Command, args []string) {
		server.StartServer(bind, port, Version)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVarP(&bind, "bind", "b", "0.0.0.0", `interface to which the server will bind`)
	serverCmd.Flags().IntVarP(&port, "port", "p", 8080, `port on which the server will listen`)
}
