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
	ColorReq    = "#918500"
	ColorBereq  = "#0077aa"
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

	// svg-sequence XML-encodes this as an attribute value, escaping it there
	txLink := "#tx-" + string(tx.TXID)

	lastDeliverIdx := -1
	lastNestedLinkIdx := -1

	for idx, rec := range tx.Records {
		switch rec := rec.(type) {
		case vsl.VCLReturnRecord:
			if rec.GetRawValue() == "deliver" {
				lastDeliverIdx = idx
			}

		case vsl.LinkRecord:
			switch rec.Reason {
			case "retry", "restart", "bgfetch":
				// These supersede the transaction instead of nesting inside it.
			default:
				lastNestedLinkIdx = idx
			}
		default:
		}
	}

	deferHandoff := lastNestedLinkIdx > lastDeliverIdx
	deferring := false

	var pendingSteps []svgsequence.Step

	// addStep helper to optionally defer steps when nested tx steps take precedence.
	addStep := func(step svgsequence.Step) {
		if deferring {
			pendingSteps = append(pendingSteps, step)

			return
		}

		s.AddStep(step)
	}

	// flushPending draws a marker showing execution is back in tx, followed by
	// every step queued while deferring was active.
	flushPending := func() {
		if len(pendingSteps) == 0 {
			return
		}

		s.AddStep(svgsequence.Step{
			Source: V,
			Text:   "RESUME (" + string(tx.TXID) + ")",
			Color:  ColorGray,
		})

		for _, step := range pendingSteps {
			s.AddStep(step)
		}

		pendingSteps = nil
		deferring = false
	}

	for i, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), Link: txLink, WithoutBorder: true}
			s.OpenSection(string(tx.TXID), &secCfg)

		case vsl.EndRecord:
			flushPending()
			s.CloseSection()

		case vsl.VCLCallRecord:
			if cfg.IncludeCalls {
				addStep(svgsequence.Step{Source: V, Text: "call " + record.GetRawValue(), Color: ColorCall})
			}

			switch r.GetRawValue() {
			case "RECV":
				addStep(svgsequence.Step{Source: client, Target: V, Text: drawRequest(reqReceived)})

			case "HASH":
				addStep(svgsequence.Step{Source: V, Target: H, Text: "HASH"})

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

				addStep(svgsequence.Step{Source: H, Target: V, Text: s1, Color: ColorHit})

			case "MISS", "PASS":
				color := ColorGray
				if r.GetRawValue() == "MISS" {
					color = ColorWarn
				}

				addStep(svgsequence.Step{Source: H, Target: V, Text: r.GetRawValue(), Color: color})

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

				addStep(svgsequence.Step{Source: V, Target: V, Text: s1, Color: color})

			case "PIPE":
				addStep(svgsequence.Step{Source: V, Target: V, Text: "Open pipe to backend and forward request"})
				addStep(svgsequence.Step{Source: B, Target: client, Text: r.GetRawValue()})

			case "BACKEND_FETCH":
				addStep(svgsequence.Step{Source: V, Target: B, Text: drawRequest(reqProcessed)})

			case "BACKEND_RESPONSE":
				// handled at return deliver/retry
				continue

			default:
			}

		case vsl.VCLReturnRecord:
			if deferHandoff && i == lastDeliverIdx {
				deferring = true
			}

			if cfg.IncludeReturns {
				addStep(svgsequence.Step{Source: V, Text: "return " + record.GetRawValue(), Color: ColorReturn})
			}

			if (r.GetRawValue() != "deliver" || i != lastDeliverIdx) && r.GetRawValue() != "retry" {
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

				addStep(svgsequence.Step{Source: V, Target: client, Text: s1, Color: statusColor(status.GetRawValue())})

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

				addStep(svgsequence.Step{Source: B, Target: V, Text: s1, Color: color})

			case vsl.TxTypeSession:
				continue

			default:
			}

		case vsl.BackendOpenRecord:
			addStep(svgsequence.Step{
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
			reason := ""
			if record.Reason != "" || record.OptionalReason != "" {
				reason = fmt.Sprintf("\n%s %s", record.Reason, record.OptionalReason)
			}

			addStep(svgsequence.Step{
				Source: B, Target: B,
				Text: fmt.Sprintf(
					"%s\n%s%s",
					record.GetTag(),
					record.Name,
					reason,
				),
			})

		// Old varnish versions
		case vsl.BackendReuseRecord:
			addStep(svgsequence.Step{
				Source: B, Target: B,
				Text: fmt.Sprintf(
					"%s\n%s",
					record.GetTag(),
					record.Name,
				),
			})

		case vsl.FetchErrorRecord:
			addStep(svgsequence.Step{Source: B, Target: B, Text: record.GetRawValue(), Color: ColorError})

		case vsl.URLRecord:
			if cfg.TrackURLAndHost {
				addStep(svgsequence.Step{
					Source: V,
					Text:   "URL: " + record.Path() + record.QueryString(),
					Color:  ColorTrack,
				})
			}

		case vsl.HeaderRecord:
			if cfg.TrackURLAndHost {
				// Header name should be already in canonical format
				if record.Name == "Host" {
					addStep(svgsequence.Step{Source: V, Text: record.Name + ": " + record.Value, Color: ColorTrack})
				}
			}

		case vsl.VCLLogRecord:
			if cfg.IncludeVCLLogs {
				addStep(svgsequence.Step{
					Source: V,
					Text:   record.String(),
					Color:  ColorGray,
				})
			}

		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx != nil {
				switch record.Reason {
				case "retry", "restart", "bgfetch":
					// These supersede the current transaction, so close its section and start a fresh sibling one.
					s.AddStep(svgsequence.Step{
						Source: V,
						Text:   strings.ToUpper(record.Reason) + " (" + string(childTx.TXID) + ")",
						Color:  ColorGray,
					})

					flushPending()
					s.CloseSection()
					addTransactionLogs(s, ts, childTx, cfg, visited)

					secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), Link: txLink, WithoutBorder: true}
					s.OpenSection(string(tx.TXID), &secCfg)

				default:
					// Nested txs like ESI includes or backend fetches.
					s.AddStep(svgsequence.Step{
						Source: V,
						Text:   strings.ToUpper(record.Reason) + " (" + string(childTx.TXID) + ")",
						Color:  ColorGray,
					})

					addTransactionLogs(s, ts, childTx, cfg, visited)
				}
			} else {
				s.AddStep(svgsequence.Step{
					Source: V,
					Text:   record.GetRawLog() + "\n*Linked child tx not found*",
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
