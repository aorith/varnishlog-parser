// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	svgsequence "github.com/aorith/svg-sequence"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

// Actors
const (
	C = "Client"
	V = "Varnish"
	H = "Cache"
	B = "Backend"
)

// Colors
const (
	ColorReq    = "#ADAD00"
	ColorBereq  = "#EE9900"
	ColorError  = "#991111"
	ColorCall   = "#555599"
	ColorReturn = "#995599"
	ColorHit    = "#115F00"
	ColorGray   = "#707070"
)

type SequenceConfig struct {
	Distance       int  // distance between actors
	StepHeight     int  // height between each step
	IncludeCalls   bool // whether to include all VCL calls
	IncludeReturns bool // whether to include all VCL returns
}

// Sequence returns a sequence diagram rendered as an SVG image.
func Sequence(ts vsl.TransactionSet, root *vsl.Transaction, cfg SequenceConfig) string {
	// Reject sessions
	if root.TXType == vsl.TxTypeSession {
		return "ERROR: sequence does not support sessions"
	}

	s := svgsequence.NewSequence()
	if cfg.Distance != 0 {
		s.SetDistance(cfg.Distance)
	}
	if cfg.StepHeight != 0 {
		s.SetStepHeight(cfg.StepHeight)
	}
	s.AddActors(C, V, H, B)

	visited := make(map[vsl.TXID]bool)
	addTransactionLogs(s, ts, root, cfg, visited)
	s.CloseAllSections()

	svg, err := s.Generate()
	if err != nil {
		return "Error: " + err.Error()
	}
	return svg
}

// addTransactionLogs is a recursive function to process each transaction's log records
// to setup the sequence diagram
func addTransactionLogs(s *svgsequence.Sequence, ts vsl.TransactionSet, tx *vsl.Transaction, cfg SequenceConfig, visited map[vsl.TXID]bool) {
	if visited[tx.TXID] {
		slog.Warn("Sequence() -> addTransactionLogs: loop detected", "transaction", tx.TXID)
		return
	}
	visited[tx.TXID] = true

	var err error
	var reqReceived, reqProcessed *HTTPRequest

	reqReceived, err = NewHTTPRequest(tx, true, nil)
	if err != nil {
		slog.Warn("failed to create HTTPRequest", "tx", tx.TXID)
		reqReceived = &HTTPRequest{}
	}
	reqProcessed, err = NewHTTPRequest(tx, false, nil)
	if err != nil {
		slog.Warn("failed to create HTTPRequest", "tx", tx.TXID)
		reqProcessed = &HTTPRequest{}
	}

	// When processing ESI, if we add the response status step at the exact moment
	// that the RespStatus record is logged, it appears before ESI processing and looks weird
	// so save it here and draw it on request end
	var respStep *svgsequence.Step

	for _, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			s.OpenSection(string(tx.TXID), getTxTypeColor(tx.TXType))

		case vsl.ReqStartRecord:
			if tx.ESILevel > 0 {
				start := fmt.Sprintf("%s\nESI Level %d", requestSequence(reqReceived), tx.ESILevel)
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: start})
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: H, Description: requestSequence(reqProcessed)})
			} else {
				start := fmt.Sprintf("%s\n%s %s", requestSequence(reqReceived), record.ClientIP, record.Listener)
				s.AddStep(svgsequence.Step{SourceActor: C, TargetActor: V, Description: start})
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: H, Description: requestSequence(reqProcessed)})
			}

		case vsl.EndRecord:
			if respStep != nil {
				s.AddStep(*respStep)
			}
			s.CloseSection()

		case vsl.SessOpenRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: C,
				TargetActor: V,
				Description: record.GetTag() + " " + record.RemoteAddr.String(),
			})

		case vsl.SessCloseRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: V,
				TargetActor: C,
				Description: record.String(),
			})

		case vsl.VCLLogRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: V,
				TargetActor: V,
				Description: record.String(),
				Color:       ColorGray,
			})

		case vsl.VCLCallRecord:
			if cfg.IncludeCalls {
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: "call " + record.GetRawValue(), Color: ColorCall})
			}

			switch r.GetRawValue() {
			case "HIT", "MISS", "PASS":
				s.AddStep(svgsequence.Step{SourceActor: H, TargetActor: V, Description: r.GetRawValue()})

			case "SYNTH":
				if respStep != nil {
					s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: "SYNTH\n" + respStep.Description})
					respStep = nil
				} else {
					s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: r.GetRawValue()})
				}

			case "PIPE":
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: C, Description: r.GetRawValue()})

			case "BACKEND_FETCH":
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: B, Description: requestSequence(reqProcessed)})

			}

		case vsl.HitRecord:
			s1 := ""
			if record.Fetched > 0 {
				s1 += "Streaming-"
			}
			s1 += record.GetTag() + "\n" + wrapAndTruncate(record.String(), 33, 120)
			s.AddStep(svgsequence.Step{SourceActor: H, TargetActor: H, Description: s1, Color: ColorHit})

		case vsl.HitMissRecord:
			s1 := fmt.Sprintf("%s TTL: %s", record.GetTag(), record.TTL.String())
			s.AddStep(svgsequence.Step{SourceActor: H, TargetActor: H, Description: s1, Color: ColorHit})

		case vsl.VCLReturnRecord:
			if cfg.IncludeReturns {
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: "return " + record.GetRawValue(), Color: ColorReturn})
			}

		case vsl.StatusRecord:
			switch record.GetTag() {
			case tags.BerespStatus:
				s1 := statusSequence(tx, record.Status, tags.BerespReason)
				respStep = &svgsequence.Step{SourceActor: B, TargetActor: V, Description: s1}

			case tags.RespStatus:
				s1 := statusSequence(tx, record.Status, tags.RespReason)
				if respStep != nil {
					// Probably a synth response
					s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: respStep.Description})
				}
				if tx.ESILevel == 0 {
					respStep = &svgsequence.Step{SourceActor: V, TargetActor: C, Description: s1}
				} else {
					respStep = &svgsequence.Step{SourceActor: V, TargetActor: V, Description: s1}
				}
			}

		case vsl.BackendOpenRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: B, TargetActor: B,
				Description: fmt.Sprintf(
					"Backend: %s\n%s %s:%d",
					truncateStrMiddle(record.Name, 40),
					record.Reason,
					record.RemoteAddr.String(),
					record.RemotePort,
				),
			})

		case vsl.FetchErrorRecord:
			s.AddStep(svgsequence.Step{SourceActor: B, TargetActor: B, Description: wrapAndTruncate(record.GetRawValue(), 35, 100), Color: ColorError})

		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx != nil {
				s.CloseSection()
				addTransactionLogs(s, ts, childTx, cfg, visited)
				s.OpenSection(string(tx.TXID), getTxTypeColor(tx.TXType))
			} else {
				actor := V
				if record.TXType == vsl.LinkTypeBereq {
					actor = B
				}
				s.AddStep(svgsequence.Step{
					SourceActor: actor, TargetActor: actor,
					Description: fmt.Sprintf("%s\n*Linked child tx not found*", record.GetRawLog()),
				})
			}
		}
	}
}

func requestSequence(req *HTTPRequest) string {
	method := req.method
	url := req.url
	host := req.host
	return method + " " + truncateStr(url, 40) + "\n" + host
}

func statusSequence(tx *vsl.Transaction, status int, reasonTag string) string {
	s := fmt.Sprintf("%d", status)
	reason := tx.RecordValueByTag(reasonTag, false)
	s += " " + wrapAndTruncate(reason, 35, 100)
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

// truncateStrMiddle trims the input string to a maximum length by keeping
// the start and end, appending "…" in the middle if it exceeds the length.
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

// wrapAndTruncate wraps text at the specified wrapLen (breaking at word boundaries)
// and truncates the final string to maxLen using truncateStr if needed.
func wrapAndTruncate(s string, wrapLen, maxLen int) string {
	if len(s) == 0 || wrapLen <= 0 {
		return truncateStr(s, maxLen)
	}

	words := strings.Fields(s)
	var builder strings.Builder
	lineLen := 0

	for i, word := range words {
		wordLen := utf8.RuneCountInString(word)

		// +1 for the space if it's not the first word in the line
		if lineLen > 0 && lineLen+1+wordLen > wrapLen {
			builder.WriteRune('\n')
			lineLen = 0
		} else if lineLen > 0 {
			builder.WriteRune(' ')
			lineLen++
		}

		builder.WriteString(word)
		lineLen += wordLen

		// Early stop if we already exceed maxLen (slightly before truncation)
		if builder.Len() > maxLen {
			break
		}

		// Avoid extra processing once near the end
		if i == len(words)-1 {
			break
		}
	}

	result := builder.String()
	if utf8.RuneCountInString(result) > maxLen {
		result = truncateStr(result, maxLen)
	}
	return result
}

// getTxTypeColor is a helper function to associate the right color to the timeline section
func getTxTypeColor(txType vsl.TxType) string {
	if txType == vsl.TxTypeBereq {
		return ColorBereq
	}
	return ColorReq
}
