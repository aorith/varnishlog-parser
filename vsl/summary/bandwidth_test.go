// SPDX-License-Identifier: MIT

package summary_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/summary"
)

func TestBandwidth(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLSimplePOST))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	report := summary.Bandwidth(ts)

	// assets/examples/simple-post.txt:
	//   ReqAcct   196 129 325 227 513 740  (rx-first: rx=196/129/325, tx=227/513/740)
	//   BereqAcct 289 129 418 118 513 631  (tx-first: tx=289/129/418, rx=118/513/631)
	want := summary.BandwidthTotals{HeaderTx: 227, BodyTx: 513, TotalTx: 740, HeaderRx: 196, BodyRx: 129, TotalRx: 325}
	if report.Client != want {
		t.Errorf("Client totals = %+v, want %+v", report.Client, want)
	}

	want = summary.BandwidthTotals{HeaderTx: 289, BodyTx: 129, TotalTx: 418, HeaderRx: 118, BodyRx: 513, TotalRx: 631}
	if report.Backend != want {
		t.Errorf("Backend totals = %+v, want %+v", report.Backend, want)
	}

	if len(report.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(report.Rows))
	}

	// Request: 325+740=1065 total bytes, BeReq: 418+631=1049 - Request sorts first.
	if report.Rows[0].TXType != vsl.TxTypeRequest {
		t.Errorf("Rows[0].TXType = %s, want %s (higher total bytes should sort first)", report.Rows[0].TXType, vsl.TxTypeRequest)
	}
}
