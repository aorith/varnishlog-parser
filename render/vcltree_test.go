// SPDX-License-Identifier: MIT

package render_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/render"
	"github.com/aorith/varnishlog-parser/vsl"
)

func TestTxTreeHTMLEscapesUntrustedValues(t *testing.T) {
	const rawLog = `*** << BeReq    >> 3
--- Begin          bereq 1 fetch
--- BereqHeader    X-Evil: "><script>alert(1)</script>
--- VCL_Log        xbody.regsub() '<Location>http://example.com/'
--- XBody          XBODY_REGSUB_<Locatio-0 0
--- End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	tx := ts.UniqueRootParents(false)[0]
	out := render.TxTreeHTML(ts, tx)

	if strings.Contains(out, "<script>") || strings.Contains(out, "<Location>") || strings.Contains(out, "<Locatio-0") {
		t.Errorf("TxTreeHTML() output contains unescaped HTML-special characters:\n%s", out)
	}

	for _, want := range []string{
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		"&lt;Location&gt;",
		"XBODY_REGSUB_&lt;Locatio-0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("TxTreeHTML() output missing escaped value %q, got: %s", want, out)
		}
	}
}

func TestTxTreeHTMLKeepsTrustedMarkup(t *testing.T) {
	const rawLog = `*** << BeReq    >> 3
--- Begin          bereq 1 fetch
--- Timestamp      Start: 1763028877.217091 0.000000 0.000000
--- TTL            RFC 120 10 0 1763028877 1763028877 1763028877 0 0 cacheable
--- End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	tx := ts.UniqueRootParents(false)[0]
	out := render.TxTreeHTML(ts, tx)

	for _, want := range []string{"<abbr title=", "</abbr>"} {
		if !strings.Contains(out, want) {
			t.Errorf("TxTreeHTML() output missing expected raw markup %q, got: %s", want, out)
		}
	}
}
