// SPDX-License-Identifier: MIT

package summary_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/summary"
)

func TestCacheStatus(t *testing.T) {
	const rawLog = `*   << Session  >> 1
-   Begin          sess 0 HTTP/1
-   End
*   << Request  >> 10
-   Begin          req 1 rxreq
-   Hit            3 5.000000 10.000000 0.000000
-   End
*   << Request  >> 11
-   Begin          req 1 rxreq
-   HitMiss        3 5.000000
-   End
*   << Request  >> 12
-   Begin          req 1 rxreq
-   HitPass        3 5.000000
-   End
*   << Request  >> 13
-   Begin          req 1 rxreq
-   End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	report := summary.CacheStatus(ts)

	want := summary.CacheStatusReport{Hit: 1, Miss: 1, HitMiss: 1, HitPass: 1}
	if report != want {
		t.Errorf("CacheStatus() = %+v, want %+v", report, want)
	}

	if report.Total() != 4 {
		t.Errorf("Total() = %d, want 4", report.Total())
	}

	if ratio := report.HitRatio(); ratio != 25 {
		t.Errorf("HitRatio() = %v, want 25", ratio)
	}
}

func TestCacheStatusEmpty(t *testing.T) {
	var ts vsl.TransactionSet

	report := summary.CacheStatus(ts)
	if report.Total() != 0 {
		t.Errorf("Total() = %d, want 0", report.Total())
	}

	if ratio := report.HitRatio(); ratio != 0 {
		t.Errorf("HitRatio() = %v, want 0", ratio)
	}
}
