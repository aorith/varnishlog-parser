package render

import (
	"fmt"
	"log"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
)

func TxTree(tx *vsl.Transaction) string {
	var s rowBuilder

	root := tx.RootParent()
	visited := make(map[string]bool)

	s.WriteString(`<ul class="root-ul">`)
	color := 0
	renderTxTree(&s, root, visited, color)
	s.WriteString("</ul>")

	return s.String()
}

func renderTxTree(s *rowBuilder, tx *vsl.Transaction, visited map[string]bool, color int) {
	if visited[tx.TXID()] {
		log.Printf("renderTxTree(): loop detected at transaction %q\n", tx.TXID())
		return
	}
	visited[tx.TXID()] = true

	s.addRow(tx.TXID(), "tx-header", "", "")

	for _, r := range tx.LogRecords() {
		switch record := r.(type) {
		case vsl.SessOpenRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.SessCloseRecord:
			s.addRow(r.Tag(), "", record.String(), "")
		case vsl.EndRecord:
			s.addRow(r.Tag(), "", "", "")
		case vsl.ReqUnsetRecord, vsl.BereqUnsetRecord, vsl.RespUnsetRecord, vsl.BerespUnsetRecord, vsl.ObjUnsetRecord:
			s.addRow(r.Tag(), "", r.Value(), "strike")
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
			childTx := tx.Children()[record.TXID()]
			if childTx == nil {
				s.addRow(r.Tag(), "", r.Value(), "strike")
				childTx = vsl.NewMissingTransaction(record)
			} else {
				s.addRow(r.Tag(), "", r.Value(), "")
			}

			s.WriteString(fmt.Sprintf(`<ul class="color-%d">`, color))
			color++
			if color > 3 {
				color = 0
			}
			renderTxTree(s, childTx, visited, color)
			s.WriteString("</ul>")
		default:
			s.addRow(r.Tag(), "", r.Value(), "")
		}
	}

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

	if classB == "" {
		classB = "tval"
	} else {
		classB = classB + " tval"
	}

	s.WriteString(fmt.Sprintf(`<div%s>%s</div>`, formatClass(classA), a))
	s.WriteString(fmt.Sprintf(`<div%s>%s</div>`, formatClass(classB), b))
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
