// SPDX-License-Identifier: MIT

// Package diagnostics inspects parsed VSL transactions for well known
// Varnish misconfigurations and errors (cache fragmentation, leaked
// state, backend/session errors, ...) and reports them as Findings.
package diagnostics

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/aorith/varnishlog-parser/vsl"
)

// Severity indicates how serious a Finding is.
type Severity int

const (
	// SeverityInfo is worth noting but often intentional.
	SeverityInfo Severity = iota
	// SeverityWarning is a likely misconfiguration or symptom of one.
	SeverityWarning
	// SeverityCritical is a bug or misconfiguration with a direct, serious
	// impact (e.g. cache poisoning, cross-user data leak, near-zero hit rate).
	SeverityCritical
)

// String returns a human-readable representation of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "Critical"
	case SeverityWarning:
		return "Warning"
	case SeverityInfo:
		return "Info"
	default:
		return "Unknown"
	}
}

// Finding represents a single detected issue, tied to the transaction it was found in.
type Finding struct {
	Rule     string     // stable machine-readable id, e.g. "vary-user-agent"
	Severity Severity   // how serious the finding is
	Summary  string     // one-line human description of the issue
	Detail   string     // specifics backing the finding: header value, count, threshold, ...
	TXID     vsl.TXID   // transaction where the issue was found
	VXID     vsl.VXID   // transaction VXID
	TXType   vsl.TxType // Session, Request or BeReq
}

// String is the string representation of a Finding.
func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s (%s): %s - %s", f.Severity, f.Rule, f.TXID, f.Summary, f.Detail)
}

func newFinding(tx *vsl.Transaction, rule string, severity Severity, summary, detail string) Finding {
	return Finding{
		Rule:     rule,
		Severity: severity,
		Summary:  summary,
		Detail:   detail,
		TXID:     tx.TXID,
		VXID:     tx.VXID,
		TXType:   tx.TXType,
	}
}

// newNonTransactionalFinding builds a Finding for a VXID 0 record.
// TXID is set to "nontransactional" to match the HTML anchor of VCL Log Tree view.
func newNonTransactionalFinding(rule string, severity Severity, summary, detail string) Finding {
	return Finding{
		Rule:     rule,
		Severity: severity,
		Summary:  summary,
		Detail:   detail,
		TXID:     "nontransactional",
	}
}

// Run inspects every transaction in the set and returns all the Findings,
// sorted by severity (most severe first) and then by VXID.
func Run(ts vsl.TransactionSet) []Finding {
	var findings []Finding // nolint:prealloc

	for _, tx := range ts.Transactions() {
		findings = append(findings, checkVary(tx)...)
		findings = append(findings, checkVaryDuplicateHeaders(tx)...)
		findings = append(findings, checkSetCookieOnHit(tx)...)
		findings = append(findings, checkAuthorizationCached(tx)...)
		findings = append(findings, checkHitForPassLongTTL(tx)...)
		findings = append(findings, checkLongGrace(tx)...)
		findings = append(findings, checkFetchError(tx)...)
		findings = append(findings, checkSingleTagMessages(tx)...)
		findings = append(findings, checkMalformedRequest(tx)...)
		findings = append(findings, checkAbnormalSessionClose(tx)...)
		findings = append(findings, checkHostHeaderCase(tx)...)
		findings = append(findings, checkGzipError(tx)...)
		findings = append(findings, checkObjectInTransientStorage(tx)...)
	}

	findings = append(findings, checkRetryStorms(ts)...)
	findings = append(findings, checkExpiryThreadPressure(ts)...)

	slices.SortStableFunc(findings, func(a, b Finding) int {
		if c := cmp.Compare(b.Severity, a.Severity); c != 0 {
			return c
		}

		return cmp.Compare(a.VXID, b.VXID)
	})

	return findings
}
