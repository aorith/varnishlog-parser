// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	svgsequence "github.com/aorith/svg-sequence"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

// Actors.
const (
	C = "Client"
	V = "Varnish"
	H = "Cache"
	B = "Backend"
)

// Colors.
const (
	ColorReq    = "#998800"
	ColorBereq  = "#008899"
	ColorError  = "#991111"
	ColorCall   = "#555599"
	ColorReturn = "#995599"
	ColorHit    = "#115F00"
	ColorGray   = "#707070"
	ColorTrack  = "#492020"
	ColorWarn   = "#a15c00" // cache misses and 4xx status codes
)

type SequenceConfig struct {
	Distance        int  // distance between actors
	StepHeight      int  // height between each step
	IncludeCalls    bool // whether to include all VCL calls
	IncludeReturns  bool // whether to include all VCL returns
	IncludeVCLLogs  bool // whether to include all VCL Logs
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
// to setup the sequence diagram.
func addTransactionLogs(s *svgsequence.Sequence, ts vsl.TransactionSet, tx *vsl.Transaction, cfg SequenceConfig, visited map[vsl.TXID]bool) { // nolint:revive
	if visited[tx.TXID] {
		slog.Warn("Sequence() -> addTransactionLogs: loop detected", "transaction", tx.TXID)

		return
	}

	visited[tx.TXID] = true

	var (
		err                       error
		reqReceived, reqProcessed *HTTPRequest
	)

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
		reqStartRecord := reqStart.(vsl.ReqStartRecord) // nolint
		client = truncateStr(reqStartRecord.ClientIP.String(), 20)
	}

	// svg-sequence XML-encodes this as an attribute value, escaping it there;
	// escaping it here too would double-escape any special characters.
	txLink := "#tx-" + string(tx.TXID)

	// vcl_hit/vcl_pass/vcl_miss can each return(deliver) on their way into vcl_deliver,
	// which itself also returns(deliver); only the last one is the actual hand-off to
	// the client/backend, so status/byte-count reporting must anchor on it.
	lastDeliverIdx := -1

	for idx, rec := range tx.Records {
		if vr, ok := rec.(vsl.VCLReturnRecord); ok && vr.GetRawValue() == "deliver" {
			lastDeliverIdx = idx
		}
	}

	for i, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), Link: txLink, WithoutBorder: true}
			s.OpenSection(string(tx.TXID), &secCfg)

		case vsl.EndRecord:
			s.CloseSection()

		case vsl.VCLCallRecord:
			if cfg.IncludeCalls {
				s.AddStep(svgsequence.Step{Source: V, Target: V, Text: "call " + record.GetRawValue(), Color: ColorCall})
			}

			switch r.GetRawValue() {
			case "RECV":
				s.AddStep(svgsequence.Step{Source: client, Target: V, Text: drawRequest(reqReceived)})

			case "HASH":
				s.AddStep(svgsequence.Step{Source: V, Target: H, Text: "HASH"})

			case "HIT":
				hitRecord := getLastHitRecord(tx, i)
				s1 := ""

				if hitRecord == nil {
					s1 = "HIT"
				} else {
					if hitRecord.Fetched > 0 {
						s1 += "Streaming-"
					}

					s1 += hitRecord.Tag + "\n"
					s1 += hitRecord.String()
				}

				s.AddStep(svgsequence.Step{Source: H, Target: V, Text: s1, Color: ColorHit})

			case "MISS", "PASS":
				color := ColorGray
				if r.GetRawValue() == "MISS" {
					color = ColorWarn
				}

				s.AddStep(svgsequence.Step{Source: H, Target: V, Text: r.GetRawValue(), Color: color})

			case "SYNTH":
				lastStatus := tx.LastRecordByTag(tags.RespStatus, i)
				lastReason := tx.LastRecordByTag(tags.RespReason, i)

				s1 := "SYNTH"
				color := ""

				if lastStatus != nil {
					s1 += "\n" + lastStatus.GetRawValue()
					color = statusColor(lastStatus.GetRawValue())
				}

				if lastReason != nil {
					s1 += " " + lastReason.GetRawValue()
				}

				s.AddStep(svgsequence.Step{Source: V, Target: V, Text: s1, Color: color})

			case "PIPE":
				s.AddStep(svgsequence.Step{Source: V, Target: V, Text: "Open pipe to backend and forward request"})
				s.AddStep(svgsequence.Step{Source: B, Target: client, Text: r.GetRawValue()})

			case "BACKEND_FETCH":
				s.AddStep(svgsequence.Step{Source: V, Target: B, Text: drawRequest(reqProcessed)})

			case "BACKEND_RESPONSE":
				// handled at return deliver
				continue

			default:
			}

		case vsl.VCLReturnRecord:
			if cfg.IncludeReturns {
				s.AddStep(svgsequence.Step{Source: V, Target: V, Text: "return " + record.GetRawValue(), Color: ColorReturn})
			}

			// Only the last return(deliver) is the actual hand-off; status/headers can
			// still be rewritten after earlier ones (e.g. vcl_hit's return(deliver)).
			if r.GetRawValue() != "deliver" || i != lastDeliverIdx {
				continue
			}

			switch tx.TXType {
			case vsl.TxTypeRequest:
				// Status/reason are read as the final value across the whole transaction
				// since Varnish can still rewrite them (e.g. 200 -> 206) after this point.
				status := tx.RecordByTag(tags.RespStatus, false)
				if status == nil {
					continue
				}

				reason := tx.RecordByTag(tags.RespReason, false)

				s1 := "DELIVER\n"
				if contentRange := tx.RespHeaders.Get("Content-Range", false); contentRange != "" {
					s1 += "Content-Range: " + contentRange + "\n"
				}

				s1 += status.GetRawValue()
				if reason != nil {
					s1 += " " + reason.GetRawValue()
				}

				acct, hasAcct := tx.RecordByTag(tags.ReqAcct, false).(vsl.AcctRecord)
				s1 += formatExtras(acct.TotalTx, hasAcct, tx.Duration())

				s.AddStep(svgsequence.Step{Source: V, Target: client, Text: s1, Color: statusColor(status.GetRawValue())})

			case vsl.TxTypeBereq:
				status := tx.RecordByTag(tags.BerespStatus, false)
				s1 := "BACKEND_RESPONSE"
				color := ""

				if status != nil {
					s1 += "\n" + status.GetRawValue()
					color = statusColor(status.GetRawValue())

					if reason := tx.RecordByTag(tags.BerespReason, false); reason != nil {
						s1 += " " + reason.GetRawValue()
					}
				}

				acct, hasAcct := tx.RecordByTag(tags.BereqAcct, false).(vsl.AcctRecord)
				s1 += formatExtras(acct.TotalRx, hasAcct, tx.Duration())

				s.AddStep(svgsequence.Step{Source: B, Target: V, Text: s1, Color: color})

			case vsl.TxTypeSession:
				continue

			default:
			}

		case vsl.BackendOpenRecord:
			s.AddStep(svgsequence.Step{
				Source: B, Target: B,
				Text: fmt.Sprintf(
					"%s\n%s\n%s %s",
					record.GetTag(),
					record.Name,
					record.Reason,
					record.ConnStr(),
				),
			})

		case vsl.BackendCloseRecord:
			s.AddStep(svgsequence.Step{
				Source: B, Target: B,
				Text: fmt.Sprintf(
					"%s\n%s\n%s %s",
					record.GetTag(),
					record.Name,
					record.Reason,
					record.OptionalReason,
				),
			})

		// Old varnish versions
		case vsl.BackendReuseRecord:
			s.AddStep(svgsequence.Step{
				Source: B, Target: B,
				Text: fmt.Sprintf(
					"%s\n%s",
					record.GetTag(),
					record.Name,
				),
			})

		case vsl.FetchErrorRecord:
			s.AddStep(svgsequence.Step{Source: B, Target: B, Text: record.GetRawValue(), Color: ColorError})

		case vsl.URLRecord:
			if cfg.TrackURLAndHost {
				s.AddStep(svgsequence.Step{
					Source: V,
					Target: V,
					Text:   "URL: " + record.Path() + record.QueryString(),
					Color:  ColorTrack,
				})
			}

		case vsl.HeaderRecord:
			if cfg.TrackURLAndHost {
				// Header name should be already in canonical format
				if record.Name == "Host" {
					s.AddStep(svgsequence.Step{Source: V, Target: V, Text: record.Name + ": " + record.Value, Color: ColorTrack})
				}
			}

		case vsl.VCLLogRecord:
			if cfg.IncludeVCLLogs {
				s.AddStep(svgsequence.Step{
					Source: V,
					Target: V,
					Text:   record.String(),
					Color:  ColorGray,
				})
			}

		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx != nil {
				switch record.Reason {
				case "retry", "restart", "bgfetch":
					// These supersede the current transaction rather than being spawned
					// from within it, so close its section and start a fresh sibling one.
					actor := V
					if record.TXType == vsl.LinkTypeBereq {
						actor = B
					}

					s.AddStep(svgsequence.Step{
						Source: actor, Target: actor,
						Text:  strings.ToUpper(record.Reason) + " (" + string(childTx.TXID) + ")",
						Color: ColorReturn,
					})

					s.CloseSection()
					addTransactionLogs(s, ts, childTx, cfg, visited)

					secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), Link: txLink, WithoutBorder: true}
					s.OpenSection(string(tx.TXID), &secCfg)

				default:
					// ESI includes, byte-range segments, backend fetches, ... happen inside
					// the current transaction: keep its section open so the child's own
					// section nests visually within it instead of splitting it in two.
					addTransactionLogs(s, ts, childTx, cfg, visited)
				}
			} else {
				actor := V
				if record.TXType == vsl.LinkTypeBereq {
					actor = B
				}

				s.AddStep(svgsequence.Step{
					Source: actor, Target: actor,
					Text: record.GetRawLog() + "\n*Linked child tx not found*",
				})
			}

		default:
		}
	}
}

func drawRequest(req *HTTPRequest) string {
	method := req.method
	url := req.url
	host := req.host

	return method + " " + url + "\n" + host
}

// truncateStr trims the input string to a maximum length, appending "…" if it exceeds the length.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	return strings.TrimSpace(string(runes[:maxLen])) + "…"
}

// formatExtras builds a "(size, duration)" suffix for a response step, omitting
// whichever part is unavailable.
func formatExtras(size vsl.SizeValue, hasSize bool, dur time.Duration) string {
	var parts []string

	if hasSize {
		parts = append(parts, size.String())
	}

	if dur > 0 {
		parts = append(parts, dur.String())
	}

	if len(parts) == 0 {
		return ""
	}

	return " (" + strings.Join(parts, ", ") + ")"
}

// statusColor highlights non-2xx/3xx HTTP status codes.
func statusColor(status string) string {
	if len(status) == 0 {
		return ""
	}

	switch status[0] {
	case '5':
		return ColorError
	case '4':
		return ColorWarn
	default:
		return ""
	}
}

// getTxTypeColor is a helper function to associate the right color to the timeline section.
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
