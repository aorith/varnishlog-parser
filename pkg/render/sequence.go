// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

// SequenceDiagram returns an string representing a Mermaid's sequence diagram
func SequenceDiagram(ts vsl.TransactionSet, tx *vsl.Transaction) string {
	var s CustomBuilder
	s.WriteString("sequenceDiagram\n")
	s.PadAdd("participant C as Client")
	s.PadAdd("participant V as Varnish")
	s.PadAdd("participant H as Cache")
	s.PadAdd("participant B as Backend")

	root := ts.RootParent(tx)
	visited := make(map[vsl.TXID]bool)
	addTransactionLogs(&s, ts, root, visited)

	return s.String()
}

// addTransactionLogs is a recursive function to process each transaction's log records
func addTransactionLogs(s *CustomBuilder, ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.TXID]bool) {
	if visited[tx.TXID()] {
		log.Printf("SequenceDiagram() -> addTransactionLogs: loop detected at transaction %q\n", tx.TXID())
		return
	}
	visited[tx.TXID()] = true

	for _, r := range tx.LogRecords() {
		switch record := r.(type) {
		case vsl.BeginRecord:
			// Type Bereq
			if tx.Type() == vsl.TxTypeBereq {
				s.PadAdd(fmt.Sprintf(
					"Note over V,B: %s",
					tx.TXID(),
				))
			}
		case vsl.SessOpenRecord:
			// Type Session
			s.PadAdd(fmt.Sprintf(
				"Note left of C: %s<br>%s",
				tx.TXID(),
				record.RemoteAddr().String(),
			))
		case vsl.ReqStartRecord:
			// Type Request
			s.PadAdd(fmt.Sprintf(
				"Note over C,H: %s<br>%s",
				tx.TXID(),
				record.ClientIP().String(),
			))

			s.PadAdd("C->>V: " + requestSequence(tx, false))
			s.PadAdd("V->>H: " + requestSequence(tx, true))
		case vsl.VCLCallRecord:
			switch r.Value() {
			case "HIT", "MISS", "PASS":
				s.PadAdd(fmt.Sprintf("H->>V: %s", r.Value()))
			case "BACKEND_FETCH":
				s.PadAdd("V->>B: " + requestSequence(tx, true))
			}
		case vsl.VCLReturnRecord:
			switch r.Value() {
			case "synth":
				s.PadAdd(fmt.Sprintf("H->>V: %s", r.Value()))
			}
		case vsl.StatusRecord:
			switch record.Tag() {
			case tag.BerespStatus:
				s1 := statusSequence(tx, record.Status(), tag.BerespReason, tag.BereqAcct)
				s.PadAdd("B->>V: " + s1)
			case tag.RespStatus:
				s1 := statusSequence(tx, record.Status(), tag.RespReason, tag.ReqAcct)
				s.PadAdd("V->>C: " + s1)
			}
		case vsl.BackendOpenRecord:
			s.PadAdd(fmt.Sprintf(
				"Note over B: %s %s<br>%s:%d",
				truncateStrMiddle(record.Name(), 65),
				record.Reason(),
				record.RemoteAddr().String(),
				record.RemotePort(),
			))
		case vsl.FetchErrorRecord:
			s.PadAdd("Note over B: " + record.Value())
		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID())
			if childTx == nil {
				if record.Type() == vsl.LinkTypeRequest {
					s.PadAdd(fmt.Sprintf("Note over V: %s<br>LINKED CHILD TX NOT FOUND IN THE LOG ", record.RawLog()))
				} else if record.Type() == vsl.LinkTypeBereq {
					s.PadAdd(fmt.Sprintf("Note over B: %s<br>LINKED CHILD TX NOT FOUND IN THE LOG ", record.RawLog()))
				}
			} else {
				addTransactionLogs(s, ts, childTx, visited)
			}
		case vsl.SessCloseRecord:
			s.PadAdd(fmt.Sprintf(
				"Note left of C: %s<br>%s (%s)",
				tx.TXID(),
				record.Reason(),
				record.Duration().String(),
			))
		}
	}
}

func requestSequence(tx *vsl.Transaction, final bool) string {
	var method, url, host string

	switch tx.Type() {
	case vsl.TxTypeSession:
		return ""
	case vsl.TxTypeRequest:
		url = tx.RecordValueByTag(tag.ReqURL, !final)
		method = tx.RecordValueByTag(tag.ReqMethod, !final)
	case vsl.TxTypeBereq:
		url = tx.RecordValueByTag(tag.BereqURL, !final)
		method = tx.RecordValueByTag(tag.BereqMethod, !final)
	}

	// TODO: logic is inverted here, why?, getting the not final header when final is true
	host = tx.ReqHeaders().Get("host", !final)

	return method + " " + truncateStr(url, 50) + "<br>" + host
}

func statusSequence(tx *vsl.Transaction, status int, reasonTag string, acctTag string) string {
	s := fmt.Sprintf("%d", status)
	reason := tx.RecordValueByTag(reasonTag, false)
	s += " " + reason
	a := tx.RecordByTag(acctTag, false)
	if a != nil {
		acct := a.(vsl.AcctRecord)
		s += fmt.Sprintf("<br>(Tx: %s | Rx: %s)", acct.TotalTx().String(), acct.TotalRx().String())
	}
	return s
}

// truncateStr trims the input string to a maximum length, appending "…" if it exceeds the length.
func truncateStr(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	runes := []rune(s)
	if maxLen > len(runes) {
		maxLen = len(runes) // Cap maxLen if it's greater than the length of runes
	}
	return strings.TrimSpace(string(runes[:maxLen])) + "…"
}

// truncateStrMiddle trims the input string to a maximum length by keeping the start and end, appending "…" in the middle if it exceeds the length.
func truncateStrMiddle(s string, maxLen int) string {
	maxLen -= 2 // Account for extra spaces
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	runes := []rune(s)
	if maxLen > len(runes) {
		maxLen = len(runes) // Cap maxLen if it's greater than the length of runes
	}

	// Split maxLen between the start and end segments
	halfLen := (maxLen - 1) / 2
	start := runes[:halfLen]
	end := runes[len(runes)-halfLen:]

	return strings.TrimSpace(string(start)) + " … " + strings.TrimSpace(string(end))
}
