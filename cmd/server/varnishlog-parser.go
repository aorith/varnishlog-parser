// SPDX-License-Identifier: MIT

// Package main starts the varnishlog-parser http server.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/aorith/varnishlog-parser/internal/server"
)

var version = "dev"

func init() { // nolint:gochecknoinits
	err := os.Setenv("TZ", "UTC")
	if err != nil {
		slog.Error("failed to set TZ=UTC", "error", err)
		os.Exit(1)
	}
}

func main() {
	// Create a new FlagSet for the server command
	bind := flag.String("bind", "0.0.0.0", "interface to which the server will bind")
	port := flag.Int("port", 8080, "port on which the server will listen")
	help := flag.Bool("help", false, "help for server")

	flag.Usage = func() {
		// nolint
		fmt.Println(`Start the http server, for example:
    server --bind 0.0.0.0 --port 8080

Usage:
    varnishlog-parser [flags]

Flags:
    --bind string   interface to which the server will bind (default "0.0.0.0")
    --port int      port on which the server will listen (default 8080)
    --help          help for server command
	`)
	}

	flag.Parse()

	if *help {
		flag.Usage()

		return
	}

	slog.Info("Starting server", "address", *bind, "port", *port)

	err := server.StartServer(*bind, *port, version)
	if err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
