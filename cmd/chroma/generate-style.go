// SPDX-License-Identifier: MIT

// Helper to generate chroma styles to highlight codeblocks
package main

import (
	"os"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

func main() {
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.WithCSSComments(false), chromahtml.ClassPrefix("chr_"))
	err := formatter.WriteCSS(os.Stdout, style)
	if err != nil {
		panic(err)
	}
}
