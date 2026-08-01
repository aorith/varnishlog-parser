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
	//go:embed examples/simple-post_g_raw.txt
	VCLSimplePOSTRaw string

	//go:embed examples/cached.txt
	VCLCached string

	//go:embed examples/streaming-hit.txt
	VCLStreamingHit string
	//go:embed examples/streaming-hit_g_raw.txt
	VCLStreamingHitRaw string

	//go:embed examples/esi-1.txt
	VCLESI1 string
	//go:embed examples/esi-1_g_raw.txt
	VCLESI1Raw string

	//go:embed examples/req-restart.txt
	VCLRestart string
	//go:embed examples/req-restart_g_raw.txt
	VCLRestartRaw string
	//go:embed examples/req-restart_verbose_g_session.txt
	VCLRestartVerboseSession string
	//go:embed examples/req-restart_verbose_g_request.txt
	VCLRestartVerboseRequest string
	//go:embed examples/req-restart_verbose_g_vxid.txt
	VCLRestartVerboseVXID string
	//go:embed examples/req-restart_verbose_g_raw.txt
	VCLRestartVerboseRaw string

	//go:embed examples/esi-synth.txt
	VCLESISynth string
	//go:embed examples/esi-synth_g_raw.txt
	VCLESISynthRaw string

	//go:embed examples/backend-retry.txt
	VCLBackendRetry string
	//go:embed examples/backend-retry_g_raw.txt
	VCLBackendRetryRaw string
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

		sb.Write(data)     //nolint:revive
		sb.WriteByte('\n') //nolint:revive
	}

	CombinedCSS = []byte(sb.String())
}
