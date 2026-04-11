// SPDX-License-Identifier: MIT

// Package summary generates a summary from the VSL transactions
package summary

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/aorith/varnishlog-parser/vsl"
)

type LatencyCounter struct {
	txType string // tx type (request, bereq)
	label  string // event label
	values []time.Duration
}

// String is the string representation of the LatencyCounter.
func (l *LatencyCounter) String() string {
	p90 := l.Percentile(90.0)
	p99 := l.Percentile(99.0)
	avg := l.Average()

	return fmt.Sprintf("[%s, %s] Count: %d | Min: %s | Max: %s | Avg: %s | P90: %s | P99: %s",
		l.txType, l.label, l.Count(), l.Min(), l.Max(), avg, p90, p99)
}

// TxType returns the transaction type that generated this event.
func (l *LatencyCounter) TxType() string {
	return l.txType
}

// Label returns the event label.
func (l *LatencyCounter) Label() string {
	return l.label
}

// Count returns the count of latencie values stored.
func (l *LatencyCounter) Count() int {
	return len(l.values)
}

// Min returns the lowest latency.
func (l *LatencyCounter) Min() time.Duration {
	if l.Count() == 0 {
		return 0
	}

	return l.values[0]
}

// Max returns the highest latency.
func (l *LatencyCounter) Max() time.Duration {
	if l.Count() == 0 {
		return 0
	}

	return l.values[len(l.values)-1]
}

// Add adds a new timestamp event to the counter.
func (l *LatencyCounter) Add(t vsl.TimestampRecord, txType string) {
	l.txType = txType
	l.label = t.EventLabel
	lat := t.SinceLast
	l.values = append(l.values, lat)
	slices.Sort(l.values) // For min, max and percentile calculations
}

// Sum computes the sum of durations.
func (l *LatencyCounter) Sum() time.Duration {
	var sum time.Duration
	for _, v := range l.values {
		sum += v
	}

	return sum
}

// Average calculates the average of durations.
func (l *LatencyCounter) Average() time.Duration {
	if len(l.values) == 0 {
		return 0
	}

	return l.Sum() / time.Duration(len(l.values))
}

// Percentile calculates a latency percentile from the stored timestamp events.
func (l *LatencyCounter) Percentile(p float64) time.Duration {
	n := len(l.values)
	if n == 0 {
		return 0
	}

	// NOTE: the slice of values is already sorted on each latencyCounter.Add() call
	// reference: https://en.wikipedia.org/wiki/Percentile
	getPercentile := func(p float64) time.Duration {
		rank := p / 100 * float64(n-1)

		// interpolate between the two closest ranks
		lowerIndex := int(rank)
		upperIndex := lowerIndex + 1

		if upperIndex >= n {
			return l.values[lowerIndex]
		}

		// linear interpolation
		weight := rank - float64(lowerIndex)

		return time.Duration(float64(l.values[lowerIndex]) + float64(weight*float64(l.values[upperIndex]-l.values[lowerIndex])))
	}

	return getPercentile(p)
}

func TimestampEventsSummary(ts vsl.TransactionSet) []*LatencyCounter {
	tsEvents := make(map[string]*LatencyCounter)

	var processEvents func(tx *vsl.Transaction)

	processEvents = func(tx *vsl.Transaction) {
		for _, r := range tx.Records {
			switch record := r.(type) {
			case vsl.TimestampRecord:
				if record.SinceLast == 0 {
					continue
				}

				name := fmt.Sprintf("%s-%s", tx.TXType, record.EventLabel)
				if tsEvents[name] == nil {
					tsEvents[name] = &LatencyCounter{}
				}

				tsEvents[name].Add(record, string(tx.TXType))

			case vsl.LinkRecord:
				child := ts.GetChildTX(tx.VXID, record.VXID)
				if child != nil {
					processEvents(child)
				}

			default:
			}
		}
	}

	for _, tx := range ts.UniqueRootParents(false) {
		processEvents(tx)
	}

	events := []*LatencyCounter{} // nolint
	for _, e := range tsEvents {
		events = append(events, e)
	}

	slices.SortStableFunc(events, func(a, b *LatencyCounter) int {
		if c := cmp.Compare(b.txType, a.txType); c != 0 {
			return c
		}

		return cmp.Compare(b.Average(), a.Average())
	})

	return events
}
