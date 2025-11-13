// SPDX-License-Identifier: MIT

package assets

import (
	"embed"
	"slices"
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

	//go:embed examples/streaming-hit.txt
	VCLStreamingHit string

	//go:embed examples/esi-1.txt
	VCLESI1 string

	//go:embed examples/req-restart.txt
	VCLRestart string

	//go:embed examples/esi-synth.txt
	VCLESISynth string
)

//go:embed all:css
var cssFiles embed.FS

var CombinedCSS []byte

func init() {
	files, err := cssFiles.ReadDir("css")
	if err != nil {
		panic(err)
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name()
	}
	slices.Sort(names)

	// Read and join all files
	var sb strings.Builder
	for _, name := range names {
		data, err := cssFiles.ReadFile("css/" + name)
		if err != nil {
			panic(err)
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	CombinedCSS = []byte(sb.String())
}
