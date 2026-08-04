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

	for _, rule := range []string{"vary-user-agent", "vary-cookie", "vary-wildcard"} {
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

func TestCacheControlPrivateOnHit(t *testing.T) {
	const rawLog = `*   << Request  >> 24
-   Begin          req 1 rxreq
-   Hit            3 5.000000 10.000000 0.000000
-   RespHeader     Cache-Control: private, max-age=0
-   End
*   << Request  >> 25
-   Begin          req 1 rxreq
-   Hit            3 5.000000 10.000000 0.000000
-   RespHeader     Cache-Control: no-store
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	var count int

	for _, f := range findings {
		if f.Rule == "cache-control-private-on-hit" {
			count++
		}
	}

	if count != 2 {
		t.Errorf("expected 2 cache-control-private-on-hit findings, got %d: %v", count, findings)
	}
}

func TestCacheControlPrivateOnHitIgnoresNormalCacheControl(t *testing.T) {
	const rawLog = `*   << Request  >> 26
-   Begin          req 1 rxreq
-   Hit            3 5.000000 10.000000 0.000000
-   RespHeader     Cache-Control: public, max-age=3600
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "cache-control-private-on-hit") {
		t.Errorf("did not expect a cache-control-private-on-hit finding, got: %v", findings)
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

func TestSingleTagMessagesRepeated(t *testing.T) {
	const rawLog = `*   << Request  >> 18
-   Begin          req 1 rxreq
-   Error          What is this 1
-   Error          What is this 2
-   Error          What is this 3
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	var count int

	for _, f := range findings {
		if f.Rule == "error-tag" {
			count++
		}
	}

	if count != 3 {
		t.Errorf("expected 3 error-tag findings (one per Error record), got %d: %v", count, findings)
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

func TestAbnormalSessionCloseNewReasons(t *testing.T) {
	for _, reason := range []string{"REQ_HTTP20", "RAPID_RESET", "BANKRUPT"} {
		rawLog := "*   << Session  >> 20\n" +
			"-   Begin          sess 0 HTTP/1\n" +
			"-   SessClose      " + reason + " 0.001\n" +
			"-   End\n"

		ts := parse(t, rawLog)
		findings := diagnostics.Run(ts)

		if len(findings) == 0 {
			t.Errorf("reason %q: expected a finding, got none", reason)
		}
	}
}

func TestSessCloseClassification(t *testing.T) {
	tests := []struct {
		reason string
		rule   string
	}{
		{"OVERLOAD", "session-close-overload"},
		{"RX_OVERFLOW", "session-close-rx-overflow"},
		{"RAPID_RESET", "session-close-rapid-reset"},
		{"BANKRUPT", "session-close-h2-bankrupt"},
	}

	for _, tt := range tests {
		rawLog := "*   << Session  >> 20\n" +
			"-   Begin          sess 0 HTTP/1\n" +
			"-   SessClose      " + tt.reason + " 0.001\n" +
			"-   End\n"

		ts := parse(t, rawLog)
		findings := diagnostics.Run(ts)

		if !hasRule(findings, tt.rule) {
			t.Errorf("reason %q: expected a %q finding, got: %v", tt.reason, tt.rule, findings)
		}
	}
}

func TestWorkspaceOverflow(t *testing.T) {
	tests := []struct {
		id   string
		rule string
	}{
		{"req", "workspace-overflow-client"},
		{"bo", "workspace-overflow-backend"},
		{"ses", "workspace-overflow-session"},
		{"wrk", "workspace-overflow-thread"},
	}

	for _, tt := range tests {
		rawLog := "*   << Request  >> 30\n" +
			"-   Begin          req 1 rxreq\n" +
			"-   Error          out of workspace (" + tt.id + ")\n" +
			"-   End\n"

		ts := parse(t, rawLog)
		findings := diagnostics.Run(ts)

		if !hasRule(findings, tt.rule) {
			t.Errorf("id %q: expected a %q finding, got: %v", tt.id, tt.rule, findings)
		}
	}
}

func TestWorkspaceOverflowDoesNotDoubleFireGenericError(t *testing.T) {
	const rawLog = `*   << Request  >> 31
-   Begin          req 1 rxreq
-   Error          out of workspace (req)
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "error-tag") {
		t.Errorf("did not expect an error-tag finding alongside workspace-overflow-client, got: %v", findings)
	}
}

func TestBogoHeaderTooManyHeaders(t *testing.T) {
	const rawLog = `*   << Request  >> 32
-   Begin          req 1 rxreq
-   BogoHeader     Too many headers: X-Extra
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "bogo-header-too-many-headers") {
		t.Errorf("expected a bogo-header-too-many-headers finding, got: %v", findings)
	}
}

func TestBogoHeaderTooLong(t *testing.T) {
	const rawLog = `*   << Request  >> 33
-   Begin          req 1 rxreq
-   BogoHeader     Header too long: X-Very-Long-Header-Value
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "bogo-header-too-long") {
		t.Errorf("expected a bogo-header-too-long finding, got: %v", findings)
	}
}

func TestBogoHeaderFallsBackToGeneric(t *testing.T) {
	const rawLog = `*   << Request  >> 34
-   Begin          req 1 rxreq
-   BogoHeader     Header without ':' X-Foo
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "malformed-request") {
		t.Errorf("expected a malformed-request finding, got: %v", findings)
	}

	if hasRule(findings, "bogo-header-too-many-headers") || hasRule(findings, "bogo-header-too-long") {
		t.Errorf("did not expect a classified bogo-header finding, got: %v", findings)
	}
}

func TestHitForMissLongTTL(t *testing.T) {
	const rawLog = `*   << Request  >> 35
-   Begin          req 1 rxreq
-   HitMiss        3 900.000000
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "hit-for-miss-long-ttl") {
		t.Errorf("expected a hit-for-miss-long-ttl finding, got: %v", findings)
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

func TestVaryDuplicateHeaders(t *testing.T) {
	// nolint: dupword
	const rawLog = `*   << Request  >> 29
-   Begin          req 1 rxreq
-   RespStatus     200
-   RespHeader     Vary: Accept-Encoding, Accept-Encoding, X-Device
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "vary-duplicate-header") {
		t.Errorf("expected a vary-duplicate-header finding, got: %v", findings)
	}
}

func TestVaryNoDuplicateHeaders(t *testing.T) {
	const rawLog = `*   << Request  >> 30
-   Begin          req 1 rxreq
-   RespStatus     200
-   RespHeader     Vary: Accept-Encoding, X-Device, Origin
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "vary-duplicate-header") {
		t.Errorf("did not expect a vary-duplicate-header finding, got: %v", findings)
	}
}

func TestGzipError(t *testing.T) {
	const rawLog = `*** << BeReq    >> 24
--- Begin          bereq 1 fetch
--- Gzip           G(un)zip error: -3 ((null))
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "gzip-error") {
		t.Errorf("expected a gzip-error finding, got: %v", findings)
	}
}

func TestGzipErrorIgnoresNormalStats(t *testing.T) {
	const rawLog = `*** << BeReq    >> 25
--- Begin          bereq 1 fetch
--- Gzip           U F E 182 159 80 80 1392
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "gzip-error") {
		t.Errorf("did not expect a gzip-error finding, got: %v", findings)
	}
}

func TestObjectInTransientStorage(t *testing.T) {
	const rawLog = `*** << BeReq    >> 26
--- Begin          bereq 1 fetch
--- TTL            RFC 120 10 0 1785584633 1785584633 1785584632 0 0 cacheable
--- Storage        malloc Transient
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if !hasRule(findings, "object-in-transient-storage") {
		t.Errorf("expected an object-in-transient-storage finding, got: %v", findings)
	}
}

func TestObjectInTransientStorageIgnoresNormalStorage(t *testing.T) {
	const rawLog = `*** << BeReq    >> 27
--- Begin          bereq 1 fetch
--- TTL            RFC 120 10 0 1785584633 1785584633 1785584632 0 0 cacheable
--- Storage        malloc s0
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "object-in-transient-storage") {
		t.Errorf("did not expect an object-in-transient-storage finding, got: %v", findings)
	}
}

func TestObjectInTransientStorageIgnoresUncacheable(t *testing.T) {
	const rawLog = `*** << BeReq    >> 28
--- Begin          bereq 1 fetch
--- TTL            VCL 120 10 0 1785584633 uncacheable
--- Storage        malloc Transient
--- End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	if hasRule(findings, "object-in-transient-storage") {
		t.Errorf("did not expect an object-in-transient-storage finding, got: %v", findings)
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

func TestExpiryThreadPressure(t *testing.T) {
	const rawLog = `0 ExpKill        - LRU_Fail
0 ExpKill        - LRU x=100
0 ExpKill        - LRU x=101
0 ExpKill        - LRU_Cand x=102 f=0x0 r=1
0 ExpKill        - EXP_Removed x=103 t=-0 h=1
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)

	for _, rule := range []string{"expiry-lru-fail", "expiry-lru-eviction"} {
		if !hasRule(findings, rule) {
			t.Errorf("expected a %q finding, got: %v", rule, findings)
		}
	}

	for _, f := range findings {
		if f.TXID != "nontransactional" {
			t.Errorf("%s: TXID = %q, want %q (should link to the non-transactional records section)", f.Rule, f.TXID, "nontransactional")
		}
	}
}

func TestGroupByRule(t *testing.T) {
	const rawLog = `*   << Request  >> 10
-   Begin          req 1 rxreq
-   RespHeader     Vary: User-Agent
-   End
*   << Request  >> 11
-   Begin          req 1 rxreq
-   RespHeader     Vary: User-Agent
-   End
*   << Request  >> 12
-   Begin          req 1 rxreq
-   RespHeader     Vary: Cookie
-   End
`

	ts := parse(t, rawLog)
	findings := diagnostics.Run(ts)
	groups := diagnostics.GroupByRule(findings)

	var uaGroup, cookieGroup *diagnostics.RuleGroup

	for i := range groups {
		switch groups[i].Rule {
		case "vary-user-agent":
			uaGroup = &groups[i]
		case "vary-cookie":
			cookieGroup = &groups[i]
		default:
		}
	}

	if uaGroup == nil {
		t.Fatalf("expected a vary-user-agent group, got: %v", groups)
	}

	if uaGroup.Count != 2 {
		t.Errorf("vary-user-agent Count = %d, want 2", uaGroup.Count)
	}

	if len(uaGroup.Examples) != 2 {
		t.Errorf("vary-user-agent Examples = %d, want 2", len(uaGroup.Examples))
	}

	if cookieGroup == nil {
		t.Fatalf("expected a vary-cookie group, got: %v", groups)
	}

	if cookieGroup.Count != 1 {
		t.Errorf("vary-cookie Count = %d, want 1", cookieGroup.Count)
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
