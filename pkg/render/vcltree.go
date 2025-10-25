// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

func TxTreeHTML(ts vsl.TransactionSet, tx *vsl.Transaction) string {
	var s rowBuilder

	root := ts.RootParent(tx)
	visited := make(map[vsl.VXID]bool)

	renderTxTree(&s, ts, root, visited)
	return s.String()
}

func renderTxTree(s *rowBuilder, ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.VXID]bool) {
	if visited[tx.VXID()] {
		log.Printf("renderTxTree(): loop detected at transaction %q\n", tx.TXID())
		return
	}
	visited[tx.VXID()] = true

	s.WriteString("<tx-logs>")

	for _, r := range tx.LogRecords() {
		switch record := r.(type) {
		case vsl.SessOpenRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.SessCloseRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.EndRecord:
			s.addRow(r.Tag(), "", "", "")
		case vsl.HeaderRecord:
			s.addRow(r.Tag(), "", record.Name()+": "+record.Value(), "")
		case vsl.HeaderUnsetRecord:
			s.addRow(r.Tag(), "", record.Name()+": "+record.Value(), "strike")
		case vsl.ErrorRecord:
			s.addRow(r.Tag(), "errorRecord", r.Value(), "errorRecord")
		case vsl.FetchErrorRecord:
			s.addRow(r.Tag(), "errorRecord", r.Value(), "errorRecord")
		case vsl.TimestampRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.TTLRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.AcctRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.HitRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.HitMissRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.GzipRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.BackendOpenRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.LengthRecord:
			s.addRow(r.Tag(), "", record.Size().String(), "")
		case vsl.VCLLogRecord:
			s.addRow(r.Tag(), "", record.String(), "logMsg")
		case vsl.StatusRecord:
			s.addRow(r.Tag(), "", r.Value(), statusCSSClass(record.Status()))
		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID())
			if childTx == nil {
				childTx = vsl.NewMissingTransaction(record)
				s.addRow(record.Tag(), "", fmt.Sprintf("%s (%s)", record.Value(), childTx.TXID()), "strike")
			} else {
				s.addRow(record.Tag(), "", fmt.Sprintf("%s (%s)", record.Value(), childTx.TXID()), "")
			}
			renderTxTree(s, ts, childTx, visited)

		default:
			s.addRow(r.Tag(), "", r.Value(), "")
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
	case tag.ReqURL, tag.BereqURL:
		return "blue"
	case tag.VCLCall:
		return "brown"
	}
	return ""
}
