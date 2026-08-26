// SPDX-License-Identifier: MIT

package vsl

import (
	"testing"

	"github.com/aorith/varnishlog-parser/vsl/tags"
)

func newTestHeaderRecord(tag, name, value string) HeaderRecord {
	return HeaderRecord{Tag: tag, Name: name, Value: value, HeaderType: tag}
}

func newTestHeaderUnsetRecord(tag, name, value string) HeaderUnsetRecord {
	return HeaderUnsetRecord{Tag: tag, Name: name, Value: value, HeaderType: tag}
}

// TestIsVarnishModifiedHeader_CanonicalMismatch is a regression test: every entry in
// varnishModifiedHeaders must match what CanonicalHeaderName actually produces for it.
// "TE" is the case that broke this before: CanonicalHeaderName("TE") is "Te", not
// "TE", so a hardcoded "TE" key would silently never match.
func TestIsVarnishModifiedHeader_CanonicalMismatch(t *testing.T) {
	for _, name := range []string{"TE", "te", "Te"} {
		if !isVarnishModifiedHeader(name, tags.ReqHeader) {
			t.Errorf("isVarnishModifiedHeader(%q, ReqHeader) = false, want true", name)
		}
	}
}

func TestHeaderTracker_PlainReceivedHeader(t *testing.T) {
	ht := newHeaderTracker(Headers{}, Headers{})

	ht.observe(newTestHeaderRecord(tags.ReqHeader, "User-Agent", "curl/8.7.1"))
	ht.observe(VCLCallRecord{})

	if got := ht.reqHeaders.Get("User-Agent", true); got != "curl/8.7.1" {
		t.Errorf("received User-Agent: want %q, got %q", "curl/8.7.1", got)
	}

	values := ht.reqHeaders.Values("User-Agent", false)
	if len(values) != 1 || values[0].State() != HdrStateReceived {
		t.Errorf("processed User-Agent: want a single Received value, got %+v", values)
	}
}

// TestHeaderTracker_XForwardedForMerge reproduces the client/received merging of a
// header that Varnish's C code accumulates and rewrites before the first VCL_call,
// e.g.:
//
//	ReqHeader X-Forwarded-For: 1.1.1.1
//	ReqHeader X-Forwarded-For: 2.2.2.2
//	ReqUnset  X-Forwarded-For: 1.1.1.1, 2.2.2.2
//	ReqHeader X-Forwarded-For: 1.1.1.1, 2.2.2.2, 192.168.65.1
//	VCL_call  RECV
func TestHeaderTracker_XForwardedForMerge(t *testing.T) {
	ht := newHeaderTracker(Headers{}, Headers{})

	ht.observe(newTestHeaderRecord(tags.ReqHeader, "X-Forwarded-For", "1.1.1.1"))
	ht.observe(newTestHeaderRecord(tags.ReqHeader, "X-Forwarded-For", "2.2.2.2"))
	ht.observe(newTestHeaderUnsetRecord(tags.ReqUnset, "X-Forwarded-For", "1.1.1.1, 2.2.2.2"))
	ht.observe(newTestHeaderRecord(tags.ReqHeader, "X-Forwarded-For", "1.1.1.1, 2.2.2.2, 192.168.65.1"))
	ht.observe(VCLCallRecord{})

	if got := ht.reqHeaders.Get("X-Forwarded-For", true); got != "1.1.1.1, 2.2.2.2" {
		t.Errorf("received X-Forwarded-For: want %q, got %q", "1.1.1.1, 2.2.2.2", got)
	}

	processed := ht.reqHeaders.Values("X-Forwarded-For", false)
	if len(processed) != 1 || processed[0].Value() != "1.1.1.1, 2.2.2.2, 192.168.65.1" || processed[0].State() != HdrStateModified {
		t.Errorf("processed X-Forwarded-For: want a single Modified merged value, got %+v", processed)
	}
}

// TestHeaderTracker_RespVarnishInjectedHeaders is a regression test: X-Varnish, Age
// and Via are injected by Varnish's own C code into the client response before
// vcl_deliver, never sent by the backend, so they must never show up as Received.
func TestHeaderTracker_RespVarnishInjectedHeaders(t *testing.T) {
	ht := newHeaderTracker(Headers{}, Headers{})

	ht.observe(StatusRecord{}) // enter the response preamble
	ht.observe(newTestHeaderRecord(tags.RespHeader, "X-Varnish", "2"))
	ht.observe(newTestHeaderRecord(tags.RespHeader, "X-Varnish", "262"))
	ht.observe(newTestHeaderRecord(tags.RespHeader, "Age", "0"))
	ht.observe(newTestHeaderRecord(tags.RespHeader, "Via", "1.1 b736436225f7 (Varnish/7.5)"))
	ht.observe(VCLCallRecord{})

	for _, name := range []string{"X-Varnish", "Age", "Via"} {
		if got := ht.respHeaders.Get(name, true); got != "" {
			t.Errorf("received %s: want empty (Varnish-injected, not from the backend), got %q", name, got)
		}

		values := ht.respHeaders.Values(name, false)
		if len(values) != 1 {
			t.Fatalf("processed %s: want a single value, got %+v", name, values)
		}

		if state := values[0].State(); state != HdrStateAdded && state != HdrStateModified {
			t.Errorf("processed %s: want Added or Modified, got %v", name, state)
		}
	}
}

func TestHeaderTracker_UnsetDeletesEvenWhenUntracked(t *testing.T) {
	ht := newHeaderTracker(Headers{}, Headers{})

	ht.observe(newTestHeaderRecord(tags.ReqHeader, "X-Custom", "foo"))
	ht.observe(newTestHeaderUnsetRecord(tags.ReqUnset, "X-Custom", "foo"))

	values := ht.reqHeaders.Values("X-Custom", false)
	if len(values) != 1 || values[0].State() != HdrStateDeleted {
		t.Errorf("X-Custom: want a single Deleted value, got %+v", values)
	}
}

func TestHeaderTracker_VCLCallWithNoHeadersIsNoop(t *testing.T) {
	ht := newHeaderTracker(Headers{}, Headers{})

	ht.observe(VCLCallRecord{})

	if len(ht.reqHeaders) != 0 || len(ht.respHeaders) != 0 {
		t.Errorf("want no headers recorded, got req=%v resp=%v", ht.reqHeaders, ht.respHeaders)
	}
}
