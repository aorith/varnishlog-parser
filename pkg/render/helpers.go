package render

import "strings"

type CustomBuilder struct {
	strings.Builder
}

// PadAdd is a helper function to append a fixed padding to the string
func (b *CustomBuilder) PadAdd(s string) {
	b.WriteString("    " + s + "\n")
}
