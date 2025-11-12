// SPDX-License-Identifier: MIT

package assets

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

//go:embed all:static
var Assets embed.FS

//go:embed all:templates
var Templates embed.FS

var (
	//go:embed examples/complete1.txt
	VCLComplete1 string

	//go:embed examples/missing-child1.txt
	VCLMissingChild1 string

	//go:embed examples/link-loop.txt
	VCLLinkLoop string

	//go:embed examples/simple-post.txt
	VCLSimplePOST string

	//go:embed examples/cached.txt
	VCLCached string

	//go:embed examples/esi-1.txt
	VCLESI1 string

	//go:embed examples/req-restart.txt
	VCLRestart string

	//go:embed examples/esi-synth.txt
	VCLESISynth string
)

var CombinedCSS []byte

func init() {
	// Get the directory of the current source file to ensure css files are found
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	currentDir := filepath.Dir(currentFile)
	cssDir := filepath.Join(currentDir, "css")

	// Generate the combined css file
	matches, err := filepath.Glob(filepath.Join(cssDir, "*.css"))
	if err != nil {
		panic(err)
	}
	sort.Strings(matches)

	var parts []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("failed to read %s: %v", path, err))
		}
		parts = append(parts, string(data))
	}
	CombinedCSS = []byte(strings.Join(parts, "\n"))
}
