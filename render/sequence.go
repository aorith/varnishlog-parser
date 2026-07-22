// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

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
func addTransactionLogs(s *svgsequence.Sequence, ts vsl.TransactionSet, tx *vsl.Transaction, cfg SequenceConfig, visited map[vsl.TXID]bool) {
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

	for i, r := range tx.Records {
		switch record := r.(type) {
		case vsl.BeginRecord:
			secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), WithoutBorder: true}
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
				s.AddStep(svgsequence.Step{Source: H, Target: V, Text: r.GetRawValue()})

			case "SYNTH":
				lastStatus := tx.LastRecordByTag(tags.RespStatus, i)
				lastReason := tx.LastRecordByTag(tags.RespReason, i)

				s1 := "SYNTH"
				if lastStatus != nil {
					s1 += "\n" + lastStatus.GetRawValue()
				}

				if lastReason != nil {
					s1 += " " + lastReason.GetRawValue()
				}

				s.AddStep(svgsequence.Step{Source: V, Target: V, Text: s1})

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

			if r.GetRawValue() != "deliver" {
				continue
			}

			switch tx.TXType {
			case vsl.TxTypeRequest:
				status := tx.LastRecordByTag(tags.RespStatus, i)
				reason := tx.LastRecordByTag(tags.RespReason, i)
				// If a RespStatus is not found, we are probably serving a cache hit
				if status != nil {
					s1 := "DELIVER\n" + status.GetRawValue()
					if reason != nil {
						s1 += " " + reason.GetRawValue()
					}

					s.AddStep(svgsequence.Step{Source: V, Target: client, Text: s1})
				}

				// Handle 200 --> 206 (partial content) by checking the next status
				status = tx.NextRecordByTag(tags.RespStatus, i)
				reason = tx.NextRecordByTag(tags.RespReason, i)
				contentRange := tx.RespHeaders.Get("Content-Range", false)

				if status != nil {
					s1 := "DELIVER\n"
					if contentRange != "" {
						s1 += "Content-Range: " + contentRange + "\n"
					}

					s1 += status.GetRawValue()
					if reason != nil {
						s1 += " " + reason.GetRawValue()
					}

					s.AddStep(svgsequence.Step{Source: V, Target: client, Text: s1})
				}

			case vsl.TxTypeBereq:
				lastStatus := tx.LastRecordByTag(tags.BerespStatus, i)
				s1 := "BACKEND_RESPONSE"

				if lastStatus != nil {
					s1 += "\n" + lastStatus.GetRawValue()

					lastReason := tx.LastRecordByTag(tags.BerespReason, i)
					if lastReason != nil {
						s1 += " " + lastReason.GetRawValue()
					}
				}

				s.AddStep(svgsequence.Step{Source: B, Target: V, Text: s1})

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
				s.CloseSection()
				addTransactionLogs(s, ts, childTx, cfg, visited)

				secCfg := svgsequence.SectionConfig{Color: getTxTypeColor(tx.TXType), WithoutBorder: true}

				s.OpenSection(string(tx.TXID), &secCfg)
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
