// SPDX-License-Identifier: MIT

package summary

import (
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

// CacheStatusReport summarizes how client requests were resolved.
type CacheStatusReport struct {
	Hit     int
	Miss    int
	HitMiss int
	HitPass int
}

// Total returns the number of client requests accounted for.
func (r CacheStatusReport) Total() int {
	return r.Hit + r.Miss + r.HitMiss + r.HitPass
}

// HitRatio returns the percentage of requests served from cache.
func (r CacheStatusReport) HitRatio() float64 {
	total := r.Total()
	if total == 0 {
		return 0
	}

	return float64(r.Hit) / float64(total) * 100
}

// CacheStatus counts cache lookup outcomes across every client request.
func CacheStatus(ts vsl.TransactionSet) CacheStatusReport {
	var report CacheStatusReport

	for _, tx := range ts.Transactions() {
		if tx.TXType != vsl.TxTypeRequest {
			continue
		}

		switch {
		case tx.RecordByTag(tags.Hit, true) != nil:
			report.Hit++
		case tx.RecordByTag(tags.HitMiss, true) != nil:
			report.HitMiss++
		case tx.RecordByTag(tags.HitPass, true) != nil:
			report.HitPass++
		default:
			report.Miss++
		}
	}

	return report
}
