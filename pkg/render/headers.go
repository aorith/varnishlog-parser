// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
)

type TableRow struct {
	Cols  []TableCol
	Class string
}

func (t TableRow) ToHTML() string {
	var sb strings.Builder

	if t.Class == "" {
		sb.WriteString("<tr>")
	} else {
		sb.WriteString(fmt.Sprintf(`<tr class="%s">`, t.Class))
	}

	for _, row := range t.Cols {
		sb.WriteString(row.ToHTML())
	}

	sb.WriteString("</tr>")
	return sb.String()
}

type TableCol struct {
	Tag   string
	Class string
	Value string
}

func (t TableCol) ToHTML() string {
	if t.Class == "" {
		return fmt.Sprintf(`<%s>%s</%s>`, t.Tag, t.Value, t.Tag)
	}
	return fmt.Sprintf(`<%s class="%s">%s</%s>`, t.Tag, t.Class, t.Value, t.Tag)
}

func HeadersTableHTML(tx *vsl.Transaction) []TableRow {
	visited := make(map[vsl.TXID]bool)
	return headersTableHTML(tx, visited)
}

func headersTableHTML(tx *vsl.Transaction, visited map[vsl.TXID]bool) []TableRow {
	if visited[tx.TXID()] {
		slog.Info("renderHeaderTree(): loop detected", "txid", tx.TXID())
		return nil
	}
	visited[tx.TXID()] = true

	var (
		reqName  string
		respName string
	)
	if tx.Type() == vsl.TxTypeBereq {
		reqName = fmt.Sprintf("BereqHeader (%s)", tx.TXID())
		respName = fmt.Sprintf("BerespHeader (%s)", tx.TXID())
	} else if tx.Type() == vsl.TxTypeRequest {
		reqName = fmt.Sprintf("ReqHeader (%s)", tx.TXID())
		respName = fmt.Sprintf("RespHeader (%s)", tx.TXID())
	}

	rows := []TableRow{}
	for _, r := range tx.LogRecords() {
		switch record := r.(type) {
		case vsl.BeginRecord:
			if tx.Type() != vsl.TxTypeSession {
				rows = append(rows, renderHeaders(tx.ReqHeaders(), reqName)...)
			}

		case vsl.LinkRecord:
			childTx := tx.Children()[record.TXID()]
			if childTx != nil {
				rows = append(rows, headersTableHTML(childTx, visited)...)
			}

		case vsl.EndRecord:
			if tx.Type() != vsl.TxTypeSession {
				rows = append(rows, renderHeaders(tx.RespHeaders(), respName)...)
			}
		}
	}

	return rows
}

func renderHeaders(headers vsl.Headers, hdrTitle string) []TableRow {
	rows := []TableRow{}
	rows = append(rows,
		TableRow{Class: "hdr-type", Cols: []TableCol{
			{Tag: "th", Value: hdrTitle},
			{Tag: "th", Value: "Received"},
			{Tag: "th", Value: "Processed"},
		}},
	)

	for _, h := range headers.GetSortedHeaders() {
		received := h.Values(true)
		processed := h.Values(false)
		numValues := max(len(received), len(processed))
		for i := range numValues {
			row := TableRow{}
			row.Cols = append(row.Cols, TableCol{Tag: "th", Class: "hdrname", Value: html.EscapeString(h.Name())})

			if i < len(received) {
				row.Cols = append(row.Cols, (renderHeader(received[i].Value(), received[i].State())))
			} else {
				row.Cols = append(row.Cols, TableCol{Tag: "td"})
			}
			if i < len(processed) {
				row.Cols = append(row.Cols, (renderHeader(processed[i].Value(), processed[i].State())))
			} else {
				row.Cols = append(row.Cols, TableCol{Tag: "td"})
			}
			rows = append(rows, row)
		}
	}

	return rows
}

func renderHeader(value string, state vsl.HdrState) TableCol {
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

	col := TableCol{Tag: "td", Value: fmt.Sprintf(`<input type="text" class="%s" disabled value="%s">`, class, value)}
	return col
}
