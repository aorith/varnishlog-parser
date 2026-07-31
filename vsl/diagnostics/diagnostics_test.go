// SPDX-License-Identifier: MIT

package diagnostics_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/diagnostics"
)

func parse(t *testing.T, rawLog string) vsl.TransactionSet {
	t.Helper()

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	return ts
}

func hasRule(findings []diagnostics.Finding, rule string) bool {
	return slices.ContainsFunc(findings, func(f diagnostics.Finding) bool {
		return f.Rule == rule
	})
}

func TestVary(t *testing.T) {
	const rawLog = `*   << Request  >> 10
-   Begin          req 1 rxreq
-   ReqMethod      GET
-   ReqURL         /a
-   RespStatus     200
-   RespHeader     Vary: User-Agent, Accept-Encoding
-   End
*   << Request  >> 11
-   Begin          req 1 rxreq
-   RespHeader     Vary: Cookie, *
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	for _, rule := range []string{"vary-user-agent", "vary-accept-encoding", "vary-cookie", "vary-wildcard"} {
		if !hasRule(findings, rule) {
			t.Errorf("expected a %q finding, got: %v", rule, findings)
		}
	}
}

func TestSetCookieOnHit(t *testing.T) {
	const rawLog = `*   << Request  >> 12
-   Begin          req 1 rxreq
-   Hit            3 5.000000 10.000000 0.000000
-   RespHeader     Set-Cookie: sessionid=abc123
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "set-cookie-on-hit") {
		t.Errorf("expected a set-cookie-on-hit finding, got: %v", findings)
	}
}

func TestAuthorizationCached(t *testing.T) {
	const rawLog = `*   << Request  >> 13
-   Begin          req 1 rxreq
-   ReqHeader      Authorization: Basic xxx
-   Hit            3 5.000000 10.000000 0.000000
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "authorization-cached") {
		t.Errorf("expected an authorization-cached finding, got: %v", findings)
	}
}

func TestHitForPassLongTTL(t *testing.T) {
	const rawLog = `*   << Request  >> 14
-   Begin          req 1 rxreq
-   HitPass        3 900.000000
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "hit-for-pass-long-ttl") {
		t.Errorf("expected a hit-for-pass-long-ttl finding, got: %v", findings)
	}
}

func TestLongGrace(t *testing.T) {
	const rawLog = `*** << BeReq    >> 15
--- Begin          bereq 1 fetch
--- TTL            RFC 120 90000 0 1763028877 1763028877 1763028877 0 0 cacheable
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "long-grace-period") {
		t.Errorf("expected a long-grace-period finding, got: %v", findings)
	}
}

func TestFetchErrorFallback(t *testing.T) {
	const rawLog = `*** << BeReq    >> 16
--- Begin          bereq 1 fetch
--- FetchError     something unexpected happened
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "fetch-error") {
		t.Errorf("expected a fetch-error finding, got: %v", findings)
	}
}

func TestFetchErrorClassification(t *testing.T) {
	tests := []struct {
		msg  string
		rule string
	}{
		{"backend notgood: fail errno 111 (Connection refused)", "fetch-error-connection-refused"},
		{"backend notgood: fail errno 104 (Connection reset by peer)", "fetch-error-connection-reset"},
		{"backend notgood: fail errno 110 (Connection timed out)", "fetch-error-connect-timeout"},
		{"backend notgood: fail errno 101 (Network is unreachable)", "fetch-error-network-unreachable"},
		{"backend notgood: fail errno 71 (Protocol error)", "fetch-error-protocol-error"},
		{"out of workspace", "fetch-error-out-of-workspace"},
		{"backend default: unhealthy", "fetch-error-backend-unhealthy"},
		{"backend default: busy", "fetch-error-backend-busy"},
		{"No thread available for bgfetch", "fetch-error-no-thread"},
		{"Director dir returned no backend", "fetch-error-no-backend"},
		{"No backend", "fetch-error-no-backend"},
		{"HTC eof (-1)", "fetch-error-htc-eof"},
		{"HTC idle (3)", "fetch-error-htc-idle"},
		{"Timed out reusing backend connection", "fetch-error-timeout"},
	}

	for _, tt := range tests {
		rawLog := "*** << BeReq    >> 16\n" +
			"--- Begin          bereq 1 fetch\n" +
			"--- FetchError     " + tt.msg + "\n" +
			"--- End\n"

		ts := parse(t, rawLog)
		findings := diagnostics.Run(ts)

		if !hasRule(findings, tt.rule) {
			t.Errorf("message %q: expected a %q finding, got: %v", tt.msg, tt.rule, findings)
		}
	}
}

func TestLostHeader(t *testing.T) {
	const rawLog = `*   << Request  >> 23
-   Begin          req 1 rxreq
-   LostHeader     X-Too-Many-Headers
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "lost-header") {
		t.Errorf("expected a lost-header finding, got: %v", findings)
	}
}

func TestESIError(t *testing.T) {
	const rawLog = `*   << Request  >> 17
-   Begin          req 1 esi 1
-   ESI_xmlerror   unmatched tag
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "esi-xml-error") {
		t.Errorf("expected an esi-xml-error finding, got: %v", findings)
	}
}

func TestVCLError(t *testing.T) {
	const rawLog = `*   << Request  >> 18
-   Begin          req 1 rxreq
-   VCL_Error      undefined symbol
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "vcl-error") {
		t.Errorf("expected a vcl-error finding, got: %v", findings)
	}
}

func TestMalformedRequest(t *testing.T) {
	const rawLog = `*   << Session  >> 19
-   Begin          sess 0 HTTP/1
-   BogoHeader     malformed header line
-   SessClose      REM_CLOSE 0.001
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "malformed-request") {
		t.Errorf("expected a malformed-request finding, got: %v", findings)
	}
}

func TestAbnormalSessionClose(t *testing.T) {
	const rawLog = `*   << Session  >> 20
-   Begin          sess 0 HTTP/1
-   SessClose      RX_TIMEOUT 30.000
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "abnormal-session-close") {
		t.Errorf("expected an abnormal-session-close finding, got: %v", findings)
	}
}

func TestAbnormalSessionCloseIgnoresNormalReasons(t *testing.T) {
	const rawLog = `*   << Session  >> 21
-   Begin          sess 0 HTTP/1
-   SessClose      REM_CLOSE 0.001
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "abnormal-session-close") {
		t.Errorf("did not expect an abnormal-session-close finding, got: %v", findings)
	}
}

func TestHostHeaderCase(t *testing.T) {
	const rawLog = `*   << Request  >> 22
-   Begin          req 1 rxreq
-   ReqHeader      Host: MyHost.Example.com
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "host-header-case") {
		t.Errorf("expected a host-header-case finding, got: %v", findings)
	}
}

func TestRetryStorm(t *testing.T) {
	const rawLog = `*   << Session  >> 1
-   Begin          sess 0 HTTP/1
-   Link           req 2 rxreq
-   SessClose      REM_CLOSE 0.001
-   End
**  << Request  >> 2
--  Begin          req 1 rxreq
--  Link           bereq 3 fetch
--  End
*** << BeReq    >> 3
--- Begin          bereq 2 fetch
--- VCL_return     retry
--- Link           bereq 4 retry
--- End
*4* << BeReq    >> 4
-4- Begin          bereq 3 retry
-4- VCL_return     retry
-4- Link           bereq 5 retry
-4- End
*5* << BeReq    >> 5
-5- Begin          bereq 4 retry
-5- VCL_return     retry
-5- Link           bereq 6 retry
-5- End
*6* << BeReq    >> 6
-6- Begin          bereq 5 retry
-6- VCL_return     deliver
-6- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "retry-storm") {
		t.Errorf("expected a retry-storm finding, got: %v", findings)
	}
}

// TestClean ensures a normal, well-behaved transaction set doesn't trigger any finding.
func TestClean(t *testing.T) {
	ts := parse(t, assets.VCLCached)

	findings := diagnostics.Run(ts)
	if len(findings) != 0 {
		t.Errorf("expected no findings on a clean log, got: %v", findings)
	}
}
