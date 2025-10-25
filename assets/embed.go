// SPDX-License-Identifier: MIT

package assets

import (
	"embed"
	"fmt"
	"os"
	"strings"
)

//go:embed all:static
var Assets embed.FS

//go:embed all:templates
var Templates embed.FS

//go:embed examples/complete1.txt
var VCLComplete1 string

//go:embed examples/missing-child1.txt
var VCLMissingChild1 string

//go:embed examples/link-loop.txt
var VCLLinkLoop string

var CombinedCSS []byte

func init() {
	// Generate the combined css file
	cssFiles := []string{
		"assets/css/vars.css",
		"assets/css/reset.css",
		"assets/css/main.css",
		"assets/css/txtree.css",
	}
	var parts []string
	for _, path := range cssFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("failed to read %s: %v", path, err))
		}
		parts = append(parts, string(data))
	}
	CombinedCSS = []byte(strings.Join(parts, "\n"))
}
