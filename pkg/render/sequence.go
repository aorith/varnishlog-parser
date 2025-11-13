// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"slices"
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
	ColorTrack  = "#492020"
)

type SequenceConfig struct {
	Distance        int  // distance between actors
	StepHeight      int  // height between each step
	IncludeCalls    bool // whether to include all VCL calls
	IncludeReturns  bool // whether to include all VCL returns
	TrackURLAndHost bool // whether to track all modifications to the URL and Host
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

	visited := make(map[vsl.TXID]bool)
	addTransactionLogs(s, ts, root, cfg, visited)

	// Ensure correct actor ordering
	finalActors := []string{}
	for _, a := range s.Actors() {
		if a != C && a != V && a != H && a != B {
			finalActors = append(finalActors, a)
		}
	}
	if slices.Contains(s.Actors(), C) {
		finalActors = append(finalActors, C)
	}
	finalActors = append(finalActors, V, H, B)
	s.AddActors(finalActors...)

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

	client := C
	reqStart := tx.NextRecordByTag(tags.ReqStart, 0)
	if reqStart != nil {
		reqStartRecord := reqStart.(vsl.ReqStartRecord)
		client = truncateStr(reqStartRecord.ClientIP.String(), 20)
	}

	truncateLen := 32
	if cfg.Distance != 0 {
		truncateLen = cfg.Distance/6 - 1
	}

	for i, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			s.OpenSection(string(tx.TXID), getTxTypeColor(tx.TXType))

		case vsl.EndRecord:
			s.CloseSection()

		case vsl.VCLCallRecord:
			if cfg.IncludeCalls {
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: "call " + record.GetRawValue(), Color: ColorCall})
			}

			switch r.GetRawValue() {
			case "RECV":
				s.AddStep(svgsequence.Step{SourceActor: client, TargetActor: V, Description: drawRequest(reqReceived, truncateLen)})

			case "HASH":
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: H, Description: "HASH"})

			case "HIT":
				hitRecord := getLastHitRecord(tx, i)
				s1 := ""
				if hitRecord == nil {
					s1 = "HIT"
				} else {
					if hitRecord.Fetched > 0 {
						s1 += "Streaming-"
					}
					s1 += fmt.Sprintf("%s\n", hitRecord.Tag)
					s1 += wrapAndTruncate(hitRecord.String(), truncateLen, 100)
				}
				s.AddStep(svgsequence.Step{SourceActor: H, TargetActor: V, Description: s1, Color: ColorHit})

			case "MISS", "PASS":
				s.AddStep(svgsequence.Step{SourceActor: H, TargetActor: V, Description: r.GetRawValue()})

			case "SYNTH":
				lastStatus := tx.LastRecordByTag(tags.RespStatus, i)
				lastReason := tx.LastRecordByTag(tags.RespReason, i)
				s1 := "SYNTH"
				if lastStatus != nil {
					s1 += "\n" + lastStatus.GetRawValue()
				}
				if lastReason != nil {
					s1 += " " + wrapAndTruncate(lastReason.GetRawValue(), truncateLen, 100)
				}
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: s1})

			case "PIPE":
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: r.GetRawValue()})

			case "BACKEND_FETCH":
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: B, Description: drawRequest(reqProcessed, truncateLen)})

			case "BACKEND_RESPONSE":
				// handled at return deliver

			}

		case vsl.VCLReturnRecord:
			if cfg.IncludeReturns {
				s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: "return " + record.GetRawValue(), Color: ColorReturn})
			}

			switch r.GetRawValue() {
			case "deliver":
				switch tx.TXType {
				case vsl.TxTypeRequest:
					lastStatus := tx.LastRecordByTag(tags.RespStatus, i)
					// If a RespStatus is not found, we are probably serving a cache hit
					if lastStatus != nil {
						s1 := "DELIVER\n" + lastStatus.GetRawValue()
						lastReason := tx.LastRecordByTag(tags.RespReason, i)
						if lastReason != nil {
							s1 += " " + wrapAndTruncate(lastReason.GetRawValue(), truncateLen, 100)
						}
						s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: client, Description: s1})
					}

				case vsl.TxTypeBereq:
					lastStatus := tx.LastRecordByTag(tags.BerespStatus, i)
					s1 := "BACKEND_RESPONSE"
					if lastStatus != nil {
						s1 += "\n" + lastStatus.GetRawValue()
						lastReason := tx.LastRecordByTag(tags.BerespReason, i)
						if lastReason != nil {
							s1 += " " + wrapAndTruncate(lastReason.GetRawValue(), truncateLen, 100)
						}
					}
					s.AddStep(svgsequence.Step{SourceActor: B, TargetActor: V, Description: s1})

				}
			}

		case vsl.BackendOpenRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: B, TargetActor: B,
				Description: fmt.Sprintf(
					"Backend: %s\n%s %s:%d",
					truncateStrMiddle(record.Name, truncateLen),
					record.Reason,
					record.RemoteAddr.String(),
					record.RemotePort,
				),
			})

		case vsl.FetchErrorRecord:
			s.AddStep(svgsequence.Step{SourceActor: B, TargetActor: B, Description: wrapAndTruncate(record.GetRawValue(), truncateLen, 100), Color: ColorError})

		case vsl.URLRecord:
			if cfg.TrackURLAndHost {
				s.AddStep(svgsequence.Step{
					SourceActor: V,
					TargetActor: V,
					Description: truncateStr("URL: "+record.Path()+record.QueryString(), truncateLen),
					Color:       ColorTrack,
				})
			}

		case vsl.HeaderRecord:
			if cfg.TrackURLAndHost {
				// Header name should be already in canonical format
				if record.Name == "Host" {
					s.AddStep(svgsequence.Step{SourceActor: V, TargetActor: V, Description: truncateStr(record.Name+": "+record.Value, truncateLen), Color: ColorTrack})
				}
			}

		case vsl.VCLLogRecord:
			s.AddStep(svgsequence.Step{
				SourceActor: V,
				TargetActor: V,
				Description: record.String(),
				Color:       ColorGray,
			})

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

func drawRequest(req *HTTPRequest, truncateLen int) string {
	method := req.method
	url := req.url
	host := req.host
	return method + " " + truncateStr(url, truncateLen) + "\n" + host
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

func getLastHitRecord(tx *vsl.Transaction, index int) *vsl.HitRecord {
	var r vsl.Record
	r = tx.LastRecordByTag(tags.Hit, index)
	if r == nil {
		r = tx.LastRecordByTag(tags.HitMiss, index)
	}
	if r == nil {
		r = tx.LastRecordByTag(tags.HitPass, index)
	}

	record, ok := r.(vsl.HitRecord)
	if ok {
		return &record
	}
	return nil
}
