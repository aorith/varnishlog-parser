// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

func TxTreeHTML(ts vsl.TransactionSet, root *vsl.Transaction) string {
	var s rowBuilder
	visited := make(map[vsl.VXID]bool)
	renderTxTree(&s, ts, root, visited)
	return s.String()
}

func renderTxTree(s *rowBuilder, ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.VXID]bool) {
	if visited[tx.VXID] {
		slog.Warn("renderTxTree(): loop detected", "transaction", tx.TXID)
		return
	}
	visited[tx.VXID] = true

	s.WriteString("<tx-logs>")

	for _, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			s.addRow(string(tx.TXID), "tx-tree-tx", "", "")
			s.addRow(record.GetTag(), "", record.GetRawValue(), "")
		case vsl.SessOpenRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.SessCloseRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.EndRecord:
			s.addRow(r.GetTag(), "", "", "")
		case vsl.HeaderRecord:
			s.addRow(r.GetTag(), "", record.Name+": "+record.Value, "")
		case vsl.HeaderUnsetRecord:
			s.addRow(r.GetTag(), "", record.Name+": "+record.Value, "strike")
		case vsl.ErrorRecord:
			s.addRow(r.GetTag(), "errorRecord", r.GetRawValue(), "errorRecord")
		case vsl.FetchErrorRecord:
			s.addRow(r.GetTag(), "errorRecord", r.GetRawValue(), "errorRecord")
		case vsl.TimestampRecord:
			s.addRow(r.GetTag(), "", timestampRecordHTML(record), "")
		case vsl.TTLRecord:
			s.addRow(r.GetTag(), "", ttlRecordHTML(record), "")
		case vsl.AcctRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.HitRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.HitMissRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.GzipRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.BackendOpenRecord:
			s.addRow(r.GetTag(), "", record.String(), "")
		case vsl.LengthRecord:
			s.addRow(r.GetTag(), "", record.Size.String(), "")
		case vsl.VCLLogRecord:
			s.addRow(r.GetTag(), "", record.String(), "logMsg")
		case vsl.StatusRecord:
			s.addRow(r.GetTag(), "", r.GetRawValue(), statusCSSClass(record.Status))
		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx == nil {
				childTx = vsl.NewMissingTransaction(record)
				s.addRow(record.GetTag(), "", fmt.Sprintf("%s (%s)", record.GetRawValue(), childTx.TXID), "strike")
			} else {
				s.addRow(record.GetTag(), "", fmt.Sprintf("%s (%s)", record.GetRawValue(), childTx.TXID), "")
			}
			renderTxTree(s, ts, childTx, visited)

		default:
			s.addRow(r.GetTag(), "", r.GetRawValue(), "")
		}
	}

	s.WriteString("</tx-logs>")
}

type rowBuilder struct {
	strings.Builder
}

func (s *rowBuilder) addRow(a, classA, b, classB string) {
	formatClass := func(cls string) string {
		if cls != "" {
			return fmt.Sprintf(` class="%s"`, cls)
		}
		return ""
	}

	if classA == "" {
		classA = keywordClass(a)
	}

	classA, classB = formatClass(classA), formatClass(classB)
	_, err := fmt.Fprintf(s, `<tx-key%s>%s</tx-key><tx-val%s>%s</tx-val>`, classA, a, classB, b)
	if err != nil {
		panic(err)
	}
}

func statusCSSClass(s int) string {
	if s >= 500 {
		return "s5xx"
	} else if s >= 400 {
		return "s4xx"
	} else if s >= 300 {
		return "s3xx"
	} else if s >= 200 {
		return "s2xx"
	}
	return ""
}

func keywordClass(s string) string {
	switch s {
	case tags.ReqURL, tags.BereqURL:
		return "blue"
	case tags.VCLCall:
		return "brown"
	case tags.VCLReturn:
		return "yellow"
	}
	return ""
}
