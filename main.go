// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/aorith/varnishlog-parser/cmd"
)

var version string = "dev"

func main() {
	if len(os.Args) < 2 {
		printMainHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "server":
		cmd.RunServer(version)
	case "-h", "--help":
		printMainHelp()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printMainHelp()
		os.Exit(1)
	}
}

func printMainHelp() {
	fmt.Println(`A varnishlog parser library and web user interface

Usage:
  varnishlog-parser [command]

Available Commands:
  server    start the http server web interface

Flags:
  --help    help for varnishlog-parser

Use "varnishlog-parser [command] --help" for more information about a command.
	`)
}
