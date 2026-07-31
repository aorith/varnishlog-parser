// SPDX-License-Identifier: MIT

package diagnostics

import (
	"fmt"
	"strings"
	"time"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

const (
	// hitForPassLongTTL flags hit-for-pass objects with a TTL well above
	// Varnish's default of 120s, since it means the uncacheable state can
	// outlive the condition (e.g. a transient Set-Cookie) that caused it.
	hitForPassLongTTL = 10 * time.Minute

	// longGrace flags objects configured to be served stale for a very long
	// time if the backend is unreachable.
	longGrace = 24 * time.Hour

	// retryStormThreshold is the number of chained retry/restart transactions
	// (bereq "retry" or req "restart") above which a request is flagged as a
	// likely missing retry-limit or unstable backend.
	retryStormThreshold = 2
)

// abnormalSessionCloseReasons are the SessClose reasons that Varnish itself
// flags as errors (the "is_error" column in varnish-cache's
// include/tbl/sess_close.h), as opposed to a client or backend simply
// closing the connection normally.
var abnormalSessionCloseReasons = map[string]bool{
	"REQ_HTTP10":    true,
	"RX_BAD":        true,
	"RX_BODY":       true,
	"RX_JUNK":       true,
	"RX_OVERFLOW":   true,
	"RX_TIMEOUT":    true,
	"TX_ERROR":      true,
	"OVERLOAD":      true,
	"PIPE_OVERFLOW": true,
	"RANGE_SHORT":   true,
	"VCL_FAILURE":   true,
}

// checkVary flags Vary header values known to fragment the cache badly
// or to make objects effectively uncacheable.
func checkVary(tx *vsl.Transaction) []Finding {
	if tx.TXType == vsl.TxTypeSession {
		return nil
	}

	vary := tx.RespHeaders.Get("Vary", false)
	if vary == "" {
		return nil
	}

	var findings []Finding

	for tok := range strings.SplitSeq(vary, ",") {
		tok = strings.TrimSpace(tok)

		switch strings.ToLower(tok) {
		case "*":
			findings = append(findings, newFinding(tx, "vary-wildcard", SeverityCritical,
				"Vary: * makes the response effectively uncacheable",
				fmt.Sprintf("Vary header: %q", vary)))
		case "user-agent":
			findings = append(findings, newFinding(tx, "vary-user-agent", SeverityCritical,
				"Vary: User-Agent fragments the cache into one object per distinct client UA string",
				fmt.Sprintf("Vary header: %q", vary)))
		case "cookie":
			findings = append(findings, newFinding(tx, "vary-cookie", SeverityWarning,
				"Vary: Cookie fragments the cache per cookie value",
				fmt.Sprintf("Vary header: %q", vary)))
		case "accept-encoding":
			findings = append(findings, newFinding(tx, "vary-accept-encoding", SeverityInfo,
				"Vary: Accept-Encoding present, verify it's normalized to a small set of values before hashing",
				fmt.Sprintf("Vary header: %q", vary)))
		default:
		}
	}

	return findings
}

// checkSetCookieOnHit flags a Set-Cookie header served from a cache hit: the
// cookie was stored in the object on the original miss and is now being
// replayed to every client that hits the same object.
func checkSetCookieOnHit(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeRequest {
		return nil
	}

	if tx.RecordByTag(tags.Hit, true) == nil {
		return nil
	}

	sc := tx.RespHeaders.Get("Set-Cookie", false)
	if sc == "" {
		return nil
	}

	return []Finding{newFinding(tx, "set-cookie-on-hit", SeverityCritical,
		"Set-Cookie header served from a cache hit, a cached cookie may be leaking to other clients",
		"Set-Cookie: "+sc)}
}

// checkAuthorizationCached flags requests that carried an Authorization
// header but were nonetheless served from cache: Varnish's default vcl_recv
// passes such requests straight to the backend, so a Hit here means custom
// VCL removed that default protection.
func checkAuthorizationCached(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeRequest {
		return nil
	}

	if tx.ReqHeaders.Get("Authorization", true) == "" {
		return nil
	}

	if tx.RecordByTag(tags.Hit, true) == nil {
		return nil
	}

	return []Finding{newFinding(tx, "authorization-cached", SeverityCritical,
		"Request carried an Authorization header but was served from cache",
		"a cache Hit occurred despite an Authorization request header")}
}

// checkHitForPassLongTTL flags hit-for-pass objects with an unusually long TTL.
func checkHitForPassLongTTL(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.HitPass, true)
	if r == nil {
		return nil
	}

	hr, ok := r.(vsl.HitRecord)
	if !ok || hr.TTL < hitForPassLongTTL {
		return nil
	}

	return []Finding{newFinding(tx, "hit-for-pass-long-ttl", SeverityWarning,
		"Hit-for-pass object has an unusually long TTL",
		fmt.Sprintf("Hit-for-pass TTL: %s (Varnish's default is 120s)", hr.TTL))}
}

// checkLongGrace flags objects configured with a very long grace period,
// which can mask a backend outage by serving stale content indefinitely.
func checkLongGrace(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.TTL, false)

	ttlRecord, ok := r.(vsl.TTLRecord)
	if !ok || ttlRecord.Grace < longGrace {
		return nil
	}

	return []Finding{newFinding(tx, "long-grace-period", SeverityInfo,
		"Object configured with a very long grace period",
		fmt.Sprintf("Grace: %s", ttlRecord.Grace))}
}

// fetchErrorClassifications maps a substring found in a FetchError message to
// a specific, actionable rule.
var fetchErrorClassifications = []struct {
	substr   string
	rule     string
	severity Severity
	summary  string
}{
	{
		"errno 111", "fetch-error-connection-refused", SeverityCritical,
		"Backend refused the connection, verify it's listening on that host/port",
	},
	{
		"errno 104", "fetch-error-connection-reset", SeverityWarning,
		"Backend reset the connection, often a TLS SNI/protocol mismatch",
	},
	{
		"errno 110", "fetch-error-connect-timeout", SeverityCritical,
		"TCP connect to the backend timed out, check connectivity/firewalls or raise connect_timeout",
	},
	{
		"errno 101", "fetch-error-network-unreachable", SeverityCritical,
		"Backend network is unreachable, check the backend address and routing",
	},
	{
		"errno 71", "fetch-error-protocol-error", SeverityWarning,
		"Protocol error talking to the backend, check for a plaintext/TLS mismatch",
	},
	{
		"out of workspace", "fetch-error-out-of-workspace", SeverityCritical,
		"Ran out of workspace processing the backend response, raise workspace_backend or check for a VMOD leak",
	},
	{
		": unhealthy", "fetch-error-backend-unhealthy", SeverityCritical,
		"Backend is marked unhealthy by its health probe, traffic to it is being diverted",
	},
	{
		": busy", "fetch-error-backend-busy", SeverityWarning,
		"Backend connection limit (max_connections) reached",
	},
	{
		"no thread available", "fetch-error-no-thread", SeverityWarning,
		"No worker thread available for the fetch, thread_pool_max may be exhausted",
	},
	{
		"no backend", "fetch-error-no-backend", SeverityCritical,
		"No backend was selected for this request, check the director/VCL backend logic or backend health",
	},
	{
		"htc eof", "fetch-error-htc-eof", SeverityInfo,
		"Backend closed an idle keep-alive connection before Varnish did, check the backend's keep-alive timeout vs backend_idle_timeout",
	},
	{
		"htc idle", "fetch-error-htc-idle", SeverityWarning,
		"Backend accepted the connection but never sent a response within first_byte_timeout",
	},
	{
		"timeout", "fetch-error-timeout", SeverityWarning,
		"Backend fetch timed out",
	},
	{
		"timed out", "fetch-error-timeout", SeverityWarning,
		"Backend fetch timed out",
	},
}

// classifyFetchError matches a FetchError message against known Varnish
// error signatures, falling back to a generic classification when nothing
// matches.
func classifyFetchError(msg string) (string, Severity, string) {
	lower := strings.ToLower(msg)

	for _, c := range fetchErrorClassifications {
		if strings.Contains(lower, c.substr) {
			return c.rule, c.severity, c.summary
		}
	}

	return "fetch-error", SeverityWarning, "Backend fetch failed"
}

// checkFetchError flags failed backend fetches, classifying well known
// Varnish error signatures (connection refused, out of workspace, unhealthy
// backend, ...) into specific, actionable rules.
func checkFetchError(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.FetchError, true)
	if r == nil {
		return nil
	}

	msg := r.GetRawValue()
	rule, severity, summary := classifyFetchError(msg)

	return []Finding{newFinding(tx, rule, severity, summary, msg)}
}

// checkLostHeader flags a header that couldn't be added to a request or
// response, typically because http_max_hdr or http_req_hdr_len/
// http_resp_hdr_len was exceeded (or, over HTTP/2, h2_max_header_list_size).
func checkLostHeader(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.LostHeader, true)
	if r == nil {
		return nil
	}

	return []Finding{newFinding(tx, "lost-header", SeverityWarning,
		"A header could not be added, exceeding a header count or size limit",
		fmt.Sprintf("Lost header: %q (check http_max_hdr, http_req_hdr_len/http_resp_hdr_len)", r.GetRawValue()))}
}

// checkESIError flags ESI parser errors or warnings.
func checkESIError(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.ESIXMLError, true)
	if r == nil {
		return nil
	}

	return []Finding{newFinding(tx, "esi-xml-error", SeverityWarning,
		"ESI parser error or warning",
		r.GetRawValue())}
}

// checkVCLError flags VCL runtime errors (e.g. calling std.* with bad
// arguments, backend errors raised from VCL, ...).
func checkVCLError(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.VCLError, true)
	if r == nil {
		return nil
	}

	return []Finding{newFinding(tx, "vcl-error", SeverityWarning,
		"VCL runtime error",
		r.GetRawValue())}
}

// checkMalformedRequest flags garbage/bogus data received on the wire,
// which is either a broken client/proxy in front of Varnish or scanning traffic.
func checkMalformedRequest(tx *vsl.Transaction) []Finding {
	malformedTags := []string{tags.BogoHeader, tags.HTTPGarbage, tags.ProxyGarbage, tags.SessError}

	var findings []Finding

	for _, tag := range malformedTags {
		r := tx.RecordByTag(tag, true)
		if r == nil {
			continue
		}

		findings = append(findings, newFinding(tx, "malformed-request", SeverityWarning,
			fmt.Sprintf("Malformed data received (%s)", tag),
			r.GetRawValue()))
	}

	return findings
}

// checkAbnormalSessionClose flags sessions closed for a reason Varnish
// itself considers an error, as opposed to a normal close.
func checkAbnormalSessionClose(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeSession {
		return nil
	}

	r := tx.RecordByTag(tags.SessClose, true)

	sc, ok := r.(vsl.SessCloseRecord)
	if !ok || !abnormalSessionCloseReasons[sc.Reason] {
		return nil
	}

	return []Finding{newFinding(tx, "abnormal-session-close", SeverityWarning,
		"Session closed for an error reason",
		fmt.Sprintf("Reason: %s, Duration: %s", sc.Reason, sc.Duration))}
}

// checkHostHeaderCase flags Host headers containing uppercase characters:
// Varnish's default hash uses req.http.host as-is, so differing casings of
// the same host fragment the cache into separate objects.
func checkHostHeaderCase(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeRequest {
		return nil
	}

	host := tx.ReqHeaders.Get("Host", true)
	if host == "" || host == strings.ToLower(host) {
		return nil
	}

	return []Finding{newFinding(tx, "host-header-case", SeverityInfo,
		"Host header contains uppercase characters, may fragment the cache",
		"Host: "+host)}
}

// checkRetryStorms flags requests chained through an unusually high number
// of bereq "retry" or req "restart" transactions.
func checkRetryStorms(ts vsl.TransactionSet) []Finding {
	var findings []Finding

	for _, group := range ts.GroupRelatedTransactions() {
		if len(group) == 0 {
			continue
		}

		var count int

		var last *vsl.Transaction

		for _, tx := range group {
			if tx.Reason == "retry" || tx.Reason == "restart" {
				count++
				last = tx
			}
		}

		if count <= retryStormThreshold {
			continue
		}

		root := group[0]
		findings = append(findings, newFinding(root, "retry-storm", SeverityWarning,
			fmt.Sprintf("Request was retried/restarted %d times", count),
			fmt.Sprintf("last retry/restart transaction: %s", last.TXID)))
	}

	return findings
}
