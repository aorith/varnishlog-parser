// SPDX-License-Identifier: MIT

package render_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/render"
	"github.com/aorith/varnishlog-parser/vsl"
)

func TestTimelineRendersEraAndTimestampEvents(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLCached))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	root := ts.UniqueRootParents(false)[0]
	svg := render.Timeline(ts, root, 1200, 10)

	if strings.HasPrefix(svg, "Error:") {
		t.Fatalf("Timeline() returned an error: %s", svg)
	}

	for _, want := range []string{"<svg", "tl-era", string(root.TXID)} {
		if !strings.Contains(svg, want) {
			t.Errorf("Timeline() output missing %q, got: %s", want, svg)
		}
	}
}

func TestTimelineLinkLoop(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLLinkLoop))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	root := ts.UniqueRootParents(false)[0]
	svg := render.Timeline(ts, root, 1200, 10)

	if strings.HasPrefix(svg, "Error:") {
		t.Fatalf("Timeline() returned an error: %s", svg)
	}
}

func TestTimelineSiblingRequestsResetRow(t *testing.T) {
	const rawLog = `*   << Session  >> 1
-   Begin          sess 0 HTTP/1
-   Link           req 2 rxreq
-   Link           req 5 rxreq
-   SessClose      REM_CLOSE 0.001
-   End
**  << Request  >> 2
--  Begin          req 1 rxreq
--  Timestamp      Start: 1700000000.000000 0.000000 0.000000
--  Timestamp      Req: 1700000000.001000 0.001000 0.001000
--  Link           bereq 3 fetch
--  Timestamp      Resp: 1700000000.010000 0.010000 0.009000
--  End
*** << BeReq    >> 3
--- Begin          bereq 2 fetch
--- Timestamp      Start: 1700000000.002000 0.002000 0.000000
--- Timestamp      Beresp: 1700000000.008000 0.008000 0.006000
--- End
*5* << Request  >> 5
-5- Begin          req 1 rxreq
-5- Timestamp      Start: 1700000000.020000 0.000000 0.000000
-5- Timestamp      Req: 1700000000.021000 0.001000 0.001000
-5- End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	root := ts.GetTX(1) // the session, so both sibling requests are walked
	svg := render.Timeline(ts, root, 1200, 10)

	if strings.HasPrefix(svg, "Error:") {
		t.Fatalf("Timeline() returned an error: %s", svg)
	}

	for _, want := range []string{"2-req-rxreq", "3-bereq-fetch", "5-req-rxreq", "ctl-e-req", "ctl-e-resp", "ctl-e-beresp"} {
		if !strings.Contains(svg, want) {
			t.Errorf("Timeline() output missing %q, got: %s", want, svg)
		}
	}
}
