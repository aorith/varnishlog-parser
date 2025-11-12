// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"html"
	"log/slog"

	"github.com/aorith/varnishlog-parser/vsl"
)

func HeadersView(ts vsl.TransactionSet, tx *vsl.Transaction) []string {
	visited := make(map[vsl.VXID]bool)
	return headersView(ts, tx, visited)
}

func headersView(ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.VXID]bool) (lines []string) {
	if visited[tx.VXID] {
		slog.Info("headersView(): loop detected", "txid", tx.TXID)
		return nil
	}
	visited[tx.VXID] = true

	for _, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			if tx.TXType != vsl.TxTypeSession {
				lines = append(lines, fmt.Sprintf(`<div class="hdr-tx">Request of %s</div>`, tx.TXID))
				lines = append(lines, renderHeaders(tx.ReqHeaders)...)
			}

		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx != nil {
				lines = append(lines, headersView(ts, childTx, visited)...)
			}

		case vsl.EndRecord:
			if tx.TXType != vsl.TxTypeSession {
				lines = append(lines, fmt.Sprintf(`<div class="hdr-tx">Response of %s</div>`, tx.TXID))
				lines = append(lines, renderHeaders(tx.RespHeaders)...)
			}
		}
	}

	return lines
}

func renderHeaders(headers vsl.Headers) (lines []string) {
	for _, t := range []string{"Header", "Received", "Processed"} {
		lines = append(lines, fmt.Sprintf(`<div class="hdr-title">%s</div>`, t))
	}

	// byte count = len(name) + len(value) + ':' + ' '
	receivedBytes := 0
	processedBytes := 0

	for _, h := range headers.GetSortedHeaders() {
		received := h.Values(true)
		processed := h.Values(false)
		numValues := max(len(received), len(processed))
		for i := range numValues {
			lines = append(lines, fmt.Sprintf(`<div class="hdr-key">%s</div>`, html.EscapeString(h.Name())))

			if i < len(received) {
				receivedBytes += len(h.Name()) + len(received[i].Value()) + 2
				lines = append(lines, renderHeader(received[i].Value(), received[i].State()))
			} else {
				lines = append(lines, `<div class="hdr-val"></div>`)
			}
			if i < len(processed) {
				processedBytes += len(h.Name()) + len(processed[i].Value()) + 2
				lines = append(lines, renderHeader(processed[i].Value(), processed[i].State()))
			} else {
				lines = append(lines, `<div class="hdr-val"></div>`)
			}
		}
	}

	lines = append(lines,
		`<div class="hdr-key hdr-bytes">Total Bytes</div>`,
		fmt.Sprintf(
			`<abbr class="hdr-bytes" title="Sum of header bytes: length(key) + length(value) + length(': ')"><input class="hdr-bytes" type="text" value="%s"></abbr>`,
			vsl.SizeValue(receivedBytes)),
		fmt.Sprintf(
			`<abbr title="Sum of header bytes: length(key) + length(value) + length(': ')"><input class="hdr-bytes"  type="text" value="%s"></abbr>`,
			vsl.SizeValue(processedBytes)),
	)

	return lines
}

func renderHeader(value string, state vsl.HdrState) string {
	size := vsl.SizeValue(len(value)) // this already accounts for multi-byte chars
	value = html.EscapeString(value)

	class := ""
	switch state {
	case vsl.HdrStateReceived:
		class = "diff-received"
	case vsl.HdrStateAdded:
		class = "diff-added"
	case vsl.HdrStateModified:
		class = "diff-modified"
	case vsl.HdrStateDeleted:
		class = "diff-deleted"
	}

	return fmt.Sprintf(`<abbr title="Size: %s"><input type="text" class="%s" value="%s"></abbr>`, size.String(), class, value)
}
