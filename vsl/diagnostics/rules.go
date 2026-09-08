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
	// uncacheableLongTTL flags hit-for-pass/hit-for-miss objects with a TTL
	// well above Varnish's default of 120s, since it means the uncacheable
	// state can outlive the condition (e.g. a transient Set-Cookie) that
	// caused it.
	uncacheableLongTTL = 10 * time.Minute

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
	"REQ_HTTP20":    true,
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
	"RAPID_RESET":   true,
	"BANKRUPT":      true,
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
		default:
		}
	}

	return findings
}

// checkVaryDuplicateHeaders flags a Vary header that lists the same header name more than once.
func checkVaryDuplicateHeaders(tx *vsl.Transaction) []Finding {
	if tx.TXType == vsl.TxTypeSession {
		return nil
	}

	vary := tx.RespHeaders.Get("Vary", false)
	if vary == "" {
		return nil
	}

	seen := make(map[string]bool)

	var duplicates []string

	for tok := range strings.SplitSeq(vary, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}

		key := strings.ToLower(tok)
		if seen[key] {
			duplicates = append(duplicates, tok)

			continue
		}

		seen[key] = true
	}

	if len(duplicates) == 0 {
		return nil
	}

	return []Finding{newFinding(tx, "vary-duplicate-header", SeverityWarning,
		"Vary header lists the same header name more than once",
		fmt.Sprintf("Duplicated: %s (Vary: %q)", strings.Join(duplicates, ", "), vary))}
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

// checkCacheControlPrivateOnHit flags a Cache-Control: private or no-store header served from a cache hit.
//
// It looks at the header's received value, i.e. as it was on the cached object
// itself, before vcl_deliver ran to avoid false positives.
func checkCacheControlPrivateOnHit(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeRequest {
		return nil
	}

	if tx.RecordByTag(tags.Hit, true) == nil {
		return nil
	}

	cc := tx.RespHeaders.Get("Cache-Control", true)

	lower := strings.ToLower(cc)
	if !strings.Contains(lower, "private") && !strings.Contains(lower, "no-store") {
		return nil
	}

	return []Finding{newFinding(tx, "cache-control-private-on-hit", SeverityCritical,
		"Cache-Control: private/no-store response served from a cache hit, a private response may be leaking to other clients",
		"Cache-Control: "+cc)}
}

// checkAuthorizationCached flags requests that carried an Authorization
// header but were served from cache.
// Varnish's default vcl_recv does pass such requests, so it is custom VCL.
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
	if !ok || hr.TTL < uncacheableLongTTL {
		return nil
	}

	return []Finding{newFinding(tx, "hit-for-pass-long-ttl", SeverityWarning,
		"Hit-for-pass object has an unusually long TTL",
		fmt.Sprintf("Hit-for-pass TTL: %s (Varnish's default is 120s)", hr.TTL))}
}

// checkHitForMissLongTTL flags hit-for-miss objects with an unusually long TTL.
func checkHitForMissLongTTL(tx *vsl.Transaction) []Finding {
	r := tx.RecordByTag(tags.HitMiss, true)
	if r == nil {
		return nil
	}

	hr, ok := r.(vsl.HitRecord)
	if !ok || hr.TTL < uncacheableLongTTL {
		return nil
	}

	return []Finding{newFinding(tx, "hit-for-miss-long-ttl", SeverityWarning,
		"Hit-for-miss object has an unusually long TTL",
		fmt.Sprintf("Hit-for-miss TTL: %s (Varnish's default is 120s)", hr.TTL))}
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

// singleTagFindings are tags that each map 1:1 to a single finding when
// present, differing only in rule id, summary and how the raw value is
// formatted into the finding's detail.
var singleTagFindings = []struct {
	tag      string
	rule     string
	severity Severity
	summary  string
	detail   func(raw string) string
}{
	{
		tags.LostHeader, "lost-header", SeverityWarning,
		"A header could not be added, exceeding a header count or size limit",
		func(raw string) string {
			return fmt.Sprintf("Lost header: %q (check http_max_hdr, http_req_hdr_len/http_resp_hdr_len)", raw)
		},
	},
	{
		tags.ESIXMLError, "esi-xml-error", SeverityWarning,
		"ESI parser error or warning",
		func(raw string) string { return raw },
	},
	{
		tags.VCLError, "vcl-error", SeverityWarning,
		"VCL runtime error",
		func(raw string) string { return raw },
	},
	{
		tags.Error, "error-tag", SeverityWarning,
		"Varnish logged an internal error",
		func(raw string) string { return raw },
	},
}

// checkSingleTagMessages flags a handful of independent tags that with
// their presence and value alone produce a finding. A transaction can log
// the same tag more than once (e.g. several Error records), so every
// occurrence is walked rather than just the first.
func checkSingleTagMessages(tx *vsl.Transaction) []Finding {
	var findings []Finding

	for _, r := range tx.Records {
		tag := r.GetTag()

		for _, c := range singleTagFindings {
			if tag != c.tag {
				continue
			}

			raw := r.GetRawValue()

			// Skip common errors
			if c.tag == tags.Error && strings.Contains(raw, "getaddrinfo() failed to resolve") {
				continue
			}

			// Workspace overflows get their own specific finding in checkWorkspaceOverflow.
			if c.tag == tags.Error && strings.HasPrefix(raw, "out of workspace") {
				continue
			}

			findings = append(findings, newFinding(tx, c.rule, c.severity, c.summary, c.detail(raw)))
		}
	}

	return findings
}

// workspaceOverflowClassifications maps the workspace id varnishd reports in
// its "out of workspace (<id>)" Error message (see http_fail() in
// varnishd's cache_http.c) to the runtime parameter that controls its size.
var workspaceOverflowClassifications = map[string]struct {
	rule    string
	summary string
}{
	"req": {"workspace-overflow-client", "Client-side workspace ran out of space, raise workspace_client"},
	"bo":  {"workspace-overflow-backend", "Backend-side workspace ran out of space, raise workspace_backend"},
	"ses": {"workspace-overflow-session", "Session workspace ran out of space, raise workspace_session"},
	"wrk": {"workspace-overflow-thread", "Worker thread workspace ran out of space, raise workspace_thread"},
}

// checkWorkspaceOverflow flags a workspace exhaustion, classified by which
// workspace (client, backend, session or worker thread) overflowed.
func checkWorkspaceOverflow(tx *vsl.Transaction) []Finding {
	var findings []Finding

	for _, r := range tx.Records {
		if r.GetTag() != tags.Error {
			continue
		}

		raw := r.GetRawValue()

		id, ok := strings.CutPrefix(raw, "out of workspace (")
		if !ok {
			continue
		}

		id = strings.TrimSuffix(id, ")")

		c, ok := workspaceOverflowClassifications[id]
		if !ok {
			continue
		}

		findings = append(findings, newFinding(tx, c.rule, SeverityCritical, c.summary, raw))
	}

	return findings
}

// checkMalformedRequest flags garbage/bogus data received.
var bogoHeaderClassifications = []struct {
	substr  string
	rule    string
	summary string
}{
	{
		"Too many headers", "bogo-header-too-many-headers",
		"Request/response exceeded the maximum header count, check http_max_hdr",
	},
	{
		"Header too long", "bogo-header-too-long",
		"A single header line was too long, check http_req_hdr_len/http_resp_hdr_len",
	},
}

// classifyBogoHeader matches a BogoHeader message against
// bogoHeaderClassifications, returning ok=false when nothing matches.
func classifyBogoHeader(msg string) (string, string, bool) {
	for _, c := range bogoHeaderClassifications {
		if strings.Contains(msg, c.substr) {
			return c.rule, c.summary, true
		}
	}

	return "", "", false
}

func checkMalformedRequest(tx *vsl.Transaction) []Finding {
	malformedTags := []string{tags.BogoHeader, tags.HTTPGarbage, tags.ProxyGarbage, tags.SessError}

	var findings []Finding

	for _, tag := range malformedTags {
		r := tx.RecordByTag(tag, true)
		if r == nil {
			continue
		}

		raw := r.GetRawValue()

		if tag == tags.BogoHeader {
			if rule, summary, ok := classifyBogoHeader(raw); ok {
				findings = append(findings, newFinding(tx, rule, SeverityWarning, summary, raw))

				continue
			}
		}

		findings = append(findings, newFinding(tx, "malformed-request", SeverityWarning,
			fmt.Sprintf("Malformed data received (%s)", tag),
			raw))
	}

	return findings
}

// sessCloseClassifications maps a subset of SessClose reasons to a specific rule.
var sessCloseClassifications = []struct {
	reason   string
	rule     string
	severity Severity
	summary  string
}{
	{
		"OVERLOAD", "session-close-overload", SeverityCritical,
		"Session closed due to resource exhaustion, likely the worker thread pools; check thread_pool_max/thread_queue_limit",
	},
	{
		"RX_OVERFLOW", "session-close-rx-overflow", SeverityWarning,
		"Request exceeded the request buffer size, check http_req_size/http_resp_size",
	},
	{
		"RAPID_RESET", "session-close-rapid-reset", SeverityCritical,
		"Possible HTTP/2 Rapid Reset attack (CVE-2023-44487), tune h2_rapid_reset_limit/h2_rapid_reset_period",
	},
	{
		"BANKRUPT", "session-close-h2-bankrupt", SeverityWarning,
		"HTTP/2 client exhausted its flow-control credit, check for a misbehaving or malicious client",
	},
}

// classifySessClose matches a SessClose reason against sessCloseClassifications.
func classifySessClose(reason string) (string, Severity, string) {
	for _, c := range sessCloseClassifications {
		if reason == c.reason {
			return c.rule, c.severity, c.summary
		}
	}

	return "abnormal-session-close", SeverityWarning, "Session closed for an error reason"
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

	rule, severity, summary := classifySessClose(sc.Reason)

	return []Finding{newFinding(tx, rule, severity, summary,
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

// expKillSubEvent returns the first whitespace-delimited token of an
// ExpKill record's raw value, e.g. "LRU_Fail" from "LRU_Fail" or "LRU" from
// "LRU x=32771".
func expKillSubEvent(raw string) string {
	sub, _, _ := strings.Cut(raw, " ")

	return sub
}

// checkExpiryThreadPressure flags signs that the expiry thread is running
// low on storage to evict from by checking non-transactional ExpKill records.
func checkExpiryThreadPressure(ts vsl.TransactionSet) []Finding {
	var lruCount, lruFailCount int

	for _, r := range ts.NonTransactional() {
		if r.GetTag() != tags.ExpKill {
			continue
		}

		switch expKillSubEvent(r.GetRawValue()) {
		case "LRU":
			lruCount++
		case "LRU_Fail":
			lruFailCount++
		default:
		}
	}

	var findings []Finding

	if lruFailCount > 0 {
		findings = append(findings, newNonTransactionalFinding("expiry-lru-fail", SeverityWarning,
			"Varnish couldn't find any object to evict under storage pressure",
			fmt.Sprintf("LRU_Fail occurred %d time(s); check storage size (malloc/file/mse) against the working set", lruFailCount)))
	}

	if lruCount > 0 {
		findings = append(findings, newNonTransactionalFinding("expiry-lru-eviction", SeverityInfo,
			"Objects were evicted from cache early under storage pressure (LRU), not via natural TTL expiry",
			fmt.Sprintf("%d object(s) force-evicted by LRU", lruCount)))
	}

	return findings
}

// checkGzipError flags a failed gzip/gunzip operation.
func checkGzipError(tx *vsl.Transaction) []Finding {
	var findings []Finding

	for _, r := range tx.Records {
		gz, ok := r.(vsl.GzipRecord)
		if !ok || gz.Error == "" {
			continue
		}

		findings = append(findings, newFinding(tx, "gzip-error", SeverityWarning,
			"Gzip/gunzip operation failed on the object body",
			gz.Error))
	}

	return findings
}

// checkObjectInTransientStorage flags a cacheable object that ended up in
// Transient storage.
func checkObjectInTransientStorage(tx *vsl.Transaction) []Finding {
	if tx.TXType != vsl.TxTypeBereq {
		return nil
	}

	storage, ok := tx.RecordByTag(tags.Storage, true).(vsl.StorageRecord)
	if !ok || storage.Name != "Transient" {
		return nil
	}

	ttl, ok := tx.RecordByTag(tags.TTL, false).(vsl.TTLRecord)
	if !ok || ttl.CacheStatus != "cacheable" {
		return nil
	}

	return []Finding{newFinding(tx, "object-in-transient-storage", SeverityWarning,
		"Cacheable object was stored in Transient storage instead of the configured storage backend",
		fmt.Sprintf("TTL: %s (source: %s); check storage size and nuke_limit", ttl.TTL, ttl.Source))}
}
