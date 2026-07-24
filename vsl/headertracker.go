// SPDX-License-Identifier: MIT

package vsl

import (
	"log/slog"

	"github.com/aorith/varnishlog-parser/vsl/tags"
)

// varnishModifiedHeaders holds the canonical names of headers known to be added,
// removed, or altered by Varnish's own C code before the first VCL_call for a given
// side (Recv for requests, Deliver for responses).
var varnishModifiedHeaders = canonicalHeaderSet(
	"Age",
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"Via",
	"X-Forwarded-For",
	"X-Varnish",
)

func canonicalHeaderSet(names ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[CanonicalHeaderName(n)] = struct{}{}
	}

	return set
}

// headerTracker reconstructs, from a transaction's ordered VSL records, which HTTP
// headers were sent as-is by the client/backend ("Received") versus added, modified,
// or deleted by Varnish's VCL or internal C code ("Processed").
//
// Varnish's VSL doesn't log this distinction directly, it only logs a header's state
// each time cache core code or VCL touches it. Varnish also has some built-in C code
// that runs before any VCL, and may add/rewrite a handful of headers (Via,
// X-Forwarded-For, X-Varnish, Age, ...) before the first VCL_call for a given side
// (request or response). So headers seen in that window are held back in `pending`
// until the VCL_call boundary is reached, then reconciled in flushPending.
//
// Ref: https://github.com/varnishcache/varnish-cache/blob/9f02342b455469349e24a88e49550f23c262baaf/bin/varnishd/cache/cache_req_fsm.c#L908-L909
//
// Example: 'Via' header with value 'a' was sent by the client:
//
//   - ReqHeader      Host: localhost:8001
//   - ReqHeader      User-Agent: curl/8.7.1
//   - ReqHeader      Accept: */*
//   - ReqHeader      Via: a
//   - ReqHeader      X-Forwarded-For: 192.168.65.1
//   - ReqUnset       Via: a
//   - ReqHeader      Via: a, 1.1 53d4be3da396 (Varnish/7.5)
//   - VCL_call       RECV
type headerTracker struct {
	reqHeaders  Headers
	respHeaders Headers

	pending    Headers // headers seen since the current preamble began, not yet attributed
	inPreamble bool    // still before the next VCL_call for the current side
	haveHeader bool    // whether any header has been observed in the current preamble
	lastIsResp bool    // side of the last header observed, valid only if haveHeader
}

// newHeaderTracker returns a headerTracker that reconciles headers directly into
// reqHeaders and respHeaders as records are observed.
func newHeaderTracker(reqHeaders, respHeaders Headers) *headerTracker {
	return &headerTracker{
		reqHeaders:  reqHeaders,
		respHeaders: respHeaders,
		pending:     make(Headers),
		inPreamble:  true,
	}
}

// observe updates header state in response to a single VSL record.
// Must be called, in order, with every record of a transaction.
func (t *headerTracker) observe(r Record) {
	switch record := r.(type) {
	case HeaderRecord:
		t.observeHeader(record)
	case HeaderUnsetRecord:
		t.observeUnset(record)
	case VCLCallRecord:
		t.observeVCLCall()
	case StatusRecord:
		// The state is back on the initial Resp/Beresp, before any VCL manipulation.
		t.inPreamble = true
	default:
	}
}

func (t *headerTracker) observeHeader(record HeaderRecord) {
	t.haveHeader = true
	t.lastIsResp = record.IsRespHeader()

	headers := t.reqHeaders
	if record.IsRespHeader() {
		headers = t.respHeaders
	}

	if !t.inPreamble {
		addProcessed(headers, record.Name, record.Value)

		return
	}

	if isVarnishModifiedHeader(record.Name, record.GetTag()) {
		// Store it to reconcile later: since deletes only apply to processed
		// headers, at flush time this should only contain the final, processed
		// values. Adding it directly to headers here would leave both a
		// client/received and a processed entry for it.
		addProcessed(t.pending, record.Name, record.Value)
	} else {
		headers.Add(record.Name, record.Value, HdrStateReceived)
	}
}

func (t *headerTracker) observeUnset(record HeaderUnsetRecord) {
	headers := t.reqHeaders
	if record.IsRespHeader() {
		headers = t.respHeaders
	}

	// All headers going forward now are considered as processed by VCL.
	if t.inPreamble {
		if isVarnishModifiedHeader(record.Name, record.GetTag()) {
			// Unset found while still in the preamble: assume we're seeing Varnish's
			// C code at work, and reconcile it once the next VCL_call is reached.
			t.pending.Add(record.Name, record.Value, HdrStateReceived)
		} else {
			slog.Warn("unset found for non-tracked Varnish C code modificable header", "header", record.Name)
		}
	}

	headers.Delete(record.Name)
	t.pending.Delete(record.Name) // received headers are not deleted
}

func (t *headerTracker) observeVCLCall() {
	if !t.inPreamble {
		return
	}

	t.inPreamble = false

	if !t.haveHeader {
		t.pending.Clear() // should be empty already

		return
	}

	// Check what was the last header to pick either reqHeaders or respHeaders,
	// rather than checking if the call is for 'recv', 'miss', 'deliver', etc,
	// as that could be more brittle.
	headers := t.reqHeaders
	if t.lastIsResp {
		headers = t.respHeaders
	}

	t.flushPending(headers)
}

// flushPending reconciles pending into headers: anything Varnish's C code received
// as-is becomes a Received header, anything it added/modified/deleted is applied as
// such. Afterwards pending is empty.
func (t *headerTracker) flushPending(headers Headers) {
	for _, h := range t.pending.GetSortedHeaders() {
		name := h.Name()

		for _, v := range h.Values(true) {
			if v.State() == HdrStateReceived {
				headers.Add(name, v.Value(), HdrStateReceived)
			}
		}

		for _, v := range h.Values(false) {
			if v.State() != HdrStateDeleted {
				headers.Add(name, v.Value(), v.State())
			} else {
				headers.Delete(name)
			}
		}
	}

	t.pending.Clear()
}

// addProcessed adds a header processed in VCL or Varnish's C code.
func addProcessed(headers Headers, name, value string) {
	if headers.Get(name, false) == "" {
		// Header does not exist, mark it as added
		headers.Add(name, value, HdrStateAdded)
	} else {
		// Header exists, add it as modified: VCL 'set' and 'unset' remove
		// all the previous values
		headers.Add(name, value, HdrStateModified)
	}
}

// isVarnishModifiedHeader checks if a header is known to be modified
// or managed internally by Varnish in its C code.
//
// This includes headers like X-Forwarded-For, Via, and others
// that Varnish may add, remove, or alter during request/response handling.
func isVarnishModifiedHeader(name, tagName string) bool {
	if name == "" {
		return false
	}

	// Only consider client-facing headers: Recv (request) and Deliver (response).
	// Bereq/Beresp are excluded since those reflect the backend transaction itself,
	// not headers received from an actual client.
	switch tagName {
	case tags.ReqHeader, tags.ReqUnset, tags.RespHeader, tags.RespUnset:
	default:
		return false
	}

	_, ok := varnishModifiedHeaders[CanonicalHeaderName(name)]

	return ok
}
