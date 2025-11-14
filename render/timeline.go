// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	svgtimeline "github.com/aorith/svg-timeline"

	"github.com/aorith/varnishlog-parser/vsl"
)

type TimelineEvent struct {
	tx        *vsl.Transaction
	record    vsl.Record
	startTime time.Time
	endTime   time.Time
	duration  time.Duration
}

// Timeline generates an SVG timeline
func Timeline(ts vsl.TransactionSet, root *vsl.Transaction, precision, numTicks int) string {
	tl := svgtimeline.NewTimeline()

	visited := make(map[vsl.VXID]bool)
	// Get the event records from all the txs and sort them by starttime
	events := collectAndSortRecords(ts, root, visited)

	var lastTx *vsl.Transaction
	txRows := make(map[vsl.VXID]int)
	currentIndex := -1
	for _, e := range events {
		switch record := e.record.(type) {

		case vsl.BeginRecord:
			if e.startTime.IsZero() || e.endTime.IsZero() {
				continue
			}

			currentIndex += 1

			if currentIndex-1 >= 0 {
				lastRow := tl.GetRowByIndex(currentIndex - 1)
				if lastRow != nil {
					rowEndTime := lastRow.EndTime()
					if e.startTime.After(rowEndTime) {
						currentIndex -= 1
					}
				}
			}

			if lastTx != nil {
				thisTxRoot := ts.RootParent(e.tx, false)
				lastTxRoot := ts.RootParent(lastTx, false)
				if thisTxRoot != nil && lastTxRoot != nil && thisTxRoot != lastTxRoot {
					// If the root tx excluding sessions is not the same, we are processing a different request transaction in the same session
					// and we should reset the row index or they will appear below in the timeline
					if ts.RootParent(e.tx, true).TXType == vsl.TxTypeSession {
						// If the root tx is a session, no further events should share its row
						currentIndex = 1
					} else {
						currentIndex = 0
					}
				}
			}
			lastTx = e.tx

			eraRow := tl.GetRowByIndex(currentIndex)
			if eraRow == nil {
				eraRow = tl.AddRow(25, 2)
			}

			eraRow.AddEvent(svgtimeline.Event{
				Type:  svgtimeline.EventTypeEra,
				Class: "ctl-" + strings.ToLower(string(e.tx.TXType)),
				Text:  string(e.tx.TXID),
				Title: fmt.Sprintf(
					"%s\nElapsed: %s\nStart Time: %s\nEnd Time: %s",
					e.tx.TXID, e.duration.String(), e.startTime.String(), e.endTime.String(),
				),
				Duration: e.duration,
				Time:     e.startTime,
			})

			if e.tx.TXType != vsl.TxTypeSession {
				// Increase the index if the current tx is not a session, since we expect timestamps records next
				currentIndex += 1
			}

		case vsl.TimestampRecord:
			var row *svgtimeline.Row = nil
			rowIndex, ok := txRows[e.tx.VXID]
			if ok {
				row = tl.GetRowByIndex(rowIndex)
			} else {
				txRows[e.tx.VXID] = currentIndex
				row = tl.GetRowByIndex(currentIndex)
			}
			if row == nil {
				row = tl.AddRow(32, 5)
			}
			row.AddEvent(
				svgtimeline.Event{
					Class: "ctl-e-" + strings.ToLower(record.EventLabel),
					Text:  record.EventLabel,
					Title: fmt.Sprintf(
						"%s (tx: %s)\nElapsed: %s\nStart Time: %s\nEnd Time: %s",
						record.EventLabel, e.tx.TXID, record.SinceLast.String(), record.StartTime.String(), record.AbsoluteTime.String(),
					),
					Duration: record.SinceLast,
					Time:     record.StartTime,
				})
		}
	}

	tl.SetPrecision(precision)
	tl.SetNumTicks(numTicks)
	tl.SetMargins(15, 30, 20, 10)
	tl.SetStyle("")

	svg, err := tl.Generate()
	if err != nil {
		return "Error: " + err.Error()
	}

	return svg
}

func collectAndSortRecords(ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.VXID]bool) (events []TimelineEvent) {
	if visited[tx.VXID] {
		slog.Info("collectAndSortRecords(): loop detected", "txid", tx.TXID)
		return
	}
	visited[tx.VXID] = true

	for _, r := range tx.Records {
		switch record := r.(type) {

		case vsl.BeginRecord:
			events = append(events, TimelineEvent{tx: tx, record: record, startTime: tx.StartTime(), endTime: tx.EndTime(), duration: tx.Duration()})

		case vsl.TimestampRecord:
			events = append(events, TimelineEvent{tx: tx, record: record, startTime: record.StartTime, endTime: record.AbsoluteTime, duration: record.SinceLast})

		case vsl.LinkRecord:
			childTx := ts.GetTX(record.VXID)
			if childTx != nil {
				events = append(events, collectAndSortRecords(ts, childTx, visited)...)
			}
		}
	}

	// Sort by startTime (keeping the same order if the value is equal, hence using stable sort)
	slices.SortStableFunc(events, func(a, b TimelineEvent) int {
		return a.startTime.Compare(b.startTime)
	})

	return events
}
