// SPDX-License-Identifier: MIT

package cmd

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/aorith/varnishlog-parser/pkg/server"
)

func RunServer(version string) {
	// Create a new FlagSet for the server command
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	bind := serverCmd.String("bind", "0.0.0.0", "interface to which the server will bind")
	port := serverCmd.Int("port", 8080, "port on which the server will listen")
	help := serverCmd.Bool("help", false, "help for server")

	serverCmd.Usage = func() {
		fmt.Println(`Start the http server, for example:
    server --bind 0.0.0.0 --port 8080

Usage:
    varnishlog-parser server [flags]

Flags:
    --bind string   interface to which the server will bind (default "0.0.0.0")
    --port int      port on which the server will listen (default 8080)
    --help          help for server command
	`)
	}

	if err := serverCmd.Parse(os.Args[2:]); err != nil {
		panic(err)
	}

	if *help {
		serverCmd.Usage()
		return
	}

	slog.Info("Starting server", "address", *bind, "port", *port)
	server.StartServer(*bind, *port, version)
}
