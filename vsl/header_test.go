// SPDX-License-Identifier: MIT

package vsl

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
)

func TestHeadersAddAndValues(t *testing.T) {
	headers := Headers{}

	// Add a single value
	headers.Add("x-test", "val1", HdrStateReceived)
	values := headers.Values("x-test", false)
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0].Value() != "val1" || values[0].State() != HdrStateReceived {
		t.Errorf("unexpected value or state: %+v", values[0])
	}

	// Add a second value for the same header
	headers.Add("x-test", "val2", HdrStateAdded)
	values = headers.Values("x-test", false)
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if values[1].Value() != "val2" || values[1].State() != HdrStateAdded {
		t.Errorf("unexpected second value: %+v", values[1])
	}

	// Add a header with HdrStateModified (should replace existing values)
	headers.Add("x-test", "val3", HdrStateModified)
	values = headers.Values("x-test", false)
	if len(values) != 1 || values[0].Value() != "val3" || values[0].State() != HdrStateModified {
		t.Errorf("HdrStateModified did not replace previous values: %+v", values)
	}

	// Add Host header (should always have unique value)
	headers.Add("Host", "example.org", HdrStateAdded)
	headers.Add("Host", "other.com", HdrStateAdded)
	values = headers.Values(HdrNameHost, false)
	if len(values) != 1 || values[0].Value() != "other.com" {
		t.Errorf("Host header did not keep unique value: %+v", values)
	}

	value := headers.Get("host", false)
	if headers.Get("host", false) != "other.com" {
		t.Errorf("unexpected host header, got: %v, wanted: %v", value, "other.com")
	}
}

func TestHeadersDelete(t *testing.T) {
	headers := Headers{}
	headers.Add("X-Test", "value1", HdrStateReceived)
	headers.Add("X-Test", "value2", HdrStateAdded)

	headers.Delete("X-Test")
	values := headers.Values("X-Test", false)
	if len(values) != 2 {
		t.Fatalf("expected 2 values after delete, got %d", len(values))
	}

	for _, v := range values {
		if v.State() != HdrStateDeleted {
			t.Errorf("expected state Deleted, got %v", v.State())
		}
	}

	// Deleting a non-existent header should not panic
	headers.Delete("Non-Existent")
}

func TestCanonicalHeaderName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"content-type", "Content-Type"},
		{"ACCEPT-encoding", "Accept-Encoding"},
		{"x-custom-header", "X-Custom-Header"},
	}

	for _, tt := range tests {
		got := CanonicalHeaderName(tt.input)
		if got != tt.want {
			t.Errorf("CanonicalHeaderName(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

type testHeader struct {
	name   string
	values []HdrValue
}

func TestHeadersFromCompleteVCL(t *testing.T) {
	ts, err := NewTransactionParser(strings.NewReader(assets.VCLComplete1)).Parse()
	if err != nil {
		t.Fatalf("vsl parser failed: %s", err)
	}

	tt1 := []testHeader{
		{name: "Host", values: []HdrValue{{value: "www.example1.org", state: HdrStateReceived}}},
		{name: "User-Agent", values: []HdrValue{{value: "curl/8.7.1", state: HdrStateReceived}}},
		{name: "Accept", values: []HdrValue{{value: "*/*", state: HdrStateReceived}}},
		{name: "Secret", values: []HdrValue{{value: "1234", state: HdrStateReceived}}},
		{name: "X-Forwarded-For", values: []HdrValue{{value: "1.1.1.1, 2.2.2.2", state: HdrStateReceived}}},
		{name: "Via", values: []HdrValue{}},
		{name: "Xid", values: []HdrValue{}},
		{name: "X-Test-Header", values: []HdrValue{}},
	}

	testHeaders(t, tt1, ts.Transactions()[1].ReqHeaders.GetSortedHeaders(), true)

	tt2 := []testHeader{
		{name: "Host", values: []HdrValue{{value: "www.example1.org", state: HdrStateReceived}}},
		{name: "User-Agent", values: []HdrValue{{value: "curl/8.7.1", state: HdrStateReceived}}},
		{name: "Accept", values: []HdrValue{{value: "*/*", state: HdrStateReceived}}},
		{name: "Secret", values: []HdrValue{{value: "1234", state: HdrStateReceived}}},
		{name: "X-Forwarded-For", values: []HdrValue{{value: "1.1.1.1, 2.2.2.2, 192.168.65.1", state: HdrStateModified}}},
		{name: "Via", values: []HdrValue{{value: "1.1 b736436225f7 (Varnish/7.5)", state: HdrStateAdded}}},
		{name: "Xid", values: []HdrValue{{value: "262", state: HdrStateAdded}}},
		{name: "X-Test-Header", values: []HdrValue{{value: "Test Value", state: HdrStateDeleted}}},
	}

	testHeaders(t, tt2, ts.Transactions()[1].ReqHeaders.GetSortedHeaders(), false)

	tt3 := []testHeader{
		{name: "Date", values: []HdrValue{{value: "Fri, 01 Nov 2024 19:59:58 GMT", state: HdrStateReceived}}},
		{name: "Server", values: []HdrValue{{value: "Varnish", state: HdrStateReceived}}},
		{name: "X-Varnish", values: []HdrValue{{value: "2", state: HdrStateReceived}, {value: "262", state: HdrStateReceived}}},
		{name: "Content-Type", values: []HdrValue{{value: "text/html; charset=utf-8", state: HdrStateReceived}}},
		{name: "Content-Length", values: []HdrValue{{value: "82", state: HdrStateReceived}}},
		{name: "Cache-Control", values: []HdrValue{{value: "max-age=5", state: HdrStateReceived}}},
		{name: "Age", values: []HdrValue{{value: "0", state: HdrStateReceived}}},
		{name: "Via", values: []HdrValue{{value: "1.1 b736436225f7 (Varnish/7.5)", state: HdrStateReceived}}},
		{name: "Accept-Ranges", values: []HdrValue{{value: "bytes", state: HdrStateReceived}}},
		{name: "X-Greet", values: []HdrValue{}},
		{name: "Connection", values: []HdrValue{}},
		{name: "Transfer-Encoding", values: []HdrValue{}},
	}

	testHeaders(t, tt3, ts.Transactions()[1].RespHeaders.GetSortedHeaders(), true)

	tt4 := []testHeader{
		{name: "Date", values: []HdrValue{{value: "Fri, 01 Nov 2024 19:59:58 GMT", state: HdrStateReceived}}},
		{name: "Server", values: []HdrValue{{value: "Varnish", state: HdrStateReceived}}},
		{name: "X-Varnish", values: []HdrValue{{value: "2", state: HdrStateReceived}, {value: "262", state: HdrStateReceived}}},
		{name: "Content-Type", values: []HdrValue{{value: "text/html; charset=utf-8", state: HdrStateReceived}}},
		{name: "Content-Length", values: []HdrValue{{value: "82", state: HdrStateDeleted}}},
		{name: "Cache-Control", values: []HdrValue{{value: "max-age=5", state: HdrStateReceived}}},
		{name: "Age", values: []HdrValue{{value: "0", state: HdrStateReceived}}},
		{name: "Via", values: []HdrValue{{value: "1.1 b736436225f7 (Varnish/7.5)", state: HdrStateReceived}}},
		{name: "Accept-Ranges", values: []HdrValue{{value: "bytes", state: HdrStateReceived}}},
		{name: "X-Greet", values: []HdrValue{{value: "Hello", state: HdrStateAdded}}},
		{name: "Connection", values: []HdrValue{{value: "keep-alive", state: HdrStateAdded}}},
		{name: "Transfer-Encoding", values: []HdrValue{{value: "chunked", state: HdrStateAdded}}},
	}

	testHeaders(t, tt4, ts.Transactions()[1].RespHeaders.GetSortedHeaders(), false)

	ts, err = NewTransactionParser(strings.NewReader(assets.VCLCached)).Parse()
	if err != nil {
		t.Fatalf("vsl parser failed: %s", err)
	}

	tt5 := []testHeader{
		{name: "Host", values: []HdrValue{{value: "varnishlog.iou.re", state: HdrStateReceived}}},
		{name: "Accept", values: []HdrValue{{value: "*/*", state: HdrStateReceived}}},
		{name: "Cached", values: []HdrValue{{value: "1", state: HdrStateReceived}}},
		{name: "User-Agent", values: []HdrValue{{value: "hurl/7.0.0", state: HdrStateReceived}}},
		{name: "X-Forwarded-For", values: []HdrValue{{value: "1.2.3.4, 192.168.65.1", state: HdrStateModified}}},
		{name: "Via", values: []HdrValue{{value: "1.1 e088e52945df (Varnish/7.7)", state: HdrStateAdded}}},
		{name: "Whoami", values: []HdrValue{{value: "1", state: HdrStateAdded}}},
	}

	testHeaders(t, tt5, ts.Transactions()[0].ReqHeaders.GetSortedHeaders(), false)
}

func testHeaders(t *testing.T, tt []testHeader, headers []Header, received bool) {
	want := []testHeader{}
	for _, v := range headers {
		th := testHeader{name: v.name, values: v.Values(received)}
		want = append(want, th)
	}

	if len(want) != len(tt) {
		t.Fatalf("ReqHeaders len; want %d, got %d", len(want), len(tt))
	}

	for i := range want {
		if want[i].name != tt[i].name {
			t.Errorf("ReqHeaders; want %v, got %v", want[i].name, tt[i].name)
		}
		if len(want[i].values) != len(tt[i].values) {
			t.Fatalf("ReqHeaders values len; %s; want %d, got %d", want[i].name, len(want[i].values), len(tt[i].values))
		}
		for j := range want[i].values {
			if want[i].values[j].value != tt[i].values[j].value || want[i].values[j].state != tt[i].values[j].state {
				t.Errorf("ReqHeaders; values[%d] %s; want %v, got %v", j, want[i].name, want[i].values[j], tt[i].values[j])
			}
		}
	}
}
