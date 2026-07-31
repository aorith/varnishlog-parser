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

// Timeline generates an SVG timeline.
func Timeline(ts vsl.TransactionSet, root *vsl.Transaction, width, numTicks int) string {
	tl := buildTimelineRows(ts, root)

	tl.SetContentWidth(width)
	tl.SetNumTicks(numTicks)
	tl.SetMargins(15, 30, 20, 10)
	tl.SetStyle("")

	svg, err := tl.Generate()
	if err != nil {
		return "Error: " + err.Error()
	}

	return svg
}

// buildTimelineRows walks every event reachable from root (sorted by start time)
// and adds era (transaction span) and timestamp rows on a new svgtimeline.Timeline.
func buildTimelineRows(ts vsl.TransactionSet, root *vsl.Transaction) *svgtimeline.Timeline {
	tl := svgtimeline.NewTimeline()

	visited := make(map[vsl.VXID]bool)
	// Get the event records from all the txs and sort them by starttime
	events := collectAndSortRecords(ts, root, visited)

	b := &timelineBuilder{ts: ts, tl: tl, txRows: make(map[vsl.VXID]int), currentIndex: -1}

	for _, e := range events {
		switch record := e.record.(type) {
		case vsl.BeginRecord:
			b.addEra(e)
		case vsl.TimestampRecord:
			b.addTimestamp(e, record)
		default:
		}
	}

	return tl
}

// timelineBuilder holds the row-placement state threaded across the events
// of a single Timeline() call.
type timelineBuilder struct {
	ts     vsl.TransactionSet
	tl     *svgtimeline.Timeline
	lastTx *vsl.Transaction
	txRows map[vsl.VXID]int

	currentIndex int
}

// addEra places a transaction's Begin/End span on an "era" row, picking the
// same row as its previous sibling request when possible, and starting a new
// row when a new, unrelated root transaction begins.
func (b *timelineBuilder) addEra(e TimelineEvent) {
	if e.startTime.IsZero() || e.endTime.IsZero() {
		return
	}

	b.currentIndex++

	if b.currentIndex-1 >= 0 {
		lastRow := b.tl.GetRowByIndex(b.currentIndex - 1)
		if lastRow != nil {
			rowEndTime := lastRow.EndTime()
			if e.startTime.After(rowEndTime) {
				b.currentIndex--
			}
		}
	}

	if b.lastTx != nil {
		thisTxRoot := b.ts.RootParent(e.tx, false)
		lastTxRoot := b.ts.RootParent(b.lastTx, false)

		if thisTxRoot != nil && lastTxRoot != nil && thisTxRoot != lastTxRoot {
			// If the root tx excluding sessions is not the same, we are processing a different request transaction in the same session
			// and we should reset the row index or they will appear below in the timeline
			if b.ts.RootParent(e.tx, true).TXType == vsl.TxTypeSession {
				// If the root tx is a session, no further events should share its row
				b.currentIndex = 1
			} else {
				b.currentIndex = 0
			}
		}
	}

	b.lastTx = e.tx

	eraRow := b.tl.GetRowByIndex(b.currentIndex)
	if eraRow == nil {
		eraRow = b.tl.AddRow(25, 2)
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
		b.currentIndex++
	}
}

// addTimestamp places a Timestamp record on the row assigned to its transaction's era
// (the first timestamp of a VXID pins the row, later ones for the same VXID reuse it).
func (b *timelineBuilder) addTimestamp(e TimelineEvent, record vsl.TimestampRecord) {
	var row *svgtimeline.Row

	rowIndex, ok := b.txRows[e.tx.VXID]
	if ok {
		row = b.tl.GetRowByIndex(rowIndex)
	} else {
		b.txRows[e.tx.VXID] = b.currentIndex
		row = b.tl.GetRowByIndex(b.currentIndex)
	}

	if row == nil {
		row = b.tl.AddRow(32, 5)
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
		},
	)
}

func collectAndSortRecords(ts vsl.TransactionSet, tx *vsl.Transaction, visited map[vsl.VXID]bool) []TimelineEvent {
	var events []TimelineEvent

	if visited[tx.VXID] {
		slog.Warn("collectAndSortRecords(): loop detected", "txid", tx.TXID)

		return events
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

		default:
		}
	}

	slices.SortStableFunc(events, func(a, b TimelineEvent) int {
		return a.startTime.Compare(b.startTime)
	})

	return events
}
