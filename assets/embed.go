package assets

import "embed"

//go:embed all:static
var Assets embed.FS

//go:embed examples/complete1.txt
var VCLComplete1 string

//go:embed examples/missing-child1.txt
var VCLMissingChild1 string

//go:embed examples/link-loop.txt
var VCLLinkLoop string
