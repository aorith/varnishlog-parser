package render

import (
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

type HTTPRequest struct {
	method  string
	host    string
	url     string
	headers []Header
}

type Header struct {
	name  string
	value string
}

type Backend struct {
	host string
	port int
}

// NewHTTPRequest constructs an HTTPRequest from a Varnish transaction.
// Returns nil if the transaction type is session.
//
// If fromResponse is true, fields are extracted from the response; otherwise, from the request.
// If received is true, initial (received) headers are used; otherwise, headers after VCL processing.
func NewHTTPRequest(tx *vsl.Transaction, fromResponse, received bool) *HTTPRequest {
	if tx.Type() == vsl.TxTypeSession {
		return nil
	}

	var headers vsl.Headers
	if fromResponse {
		headers = tx.RespHeaders()
	} else {
		headers = tx.ReqHeaders()
	}

	host := headers.Get("host", received)
	httpHeaders := []Header{}
	for name, h := range headers {
		if name == vsl.HdrNameHost {
			continue
		}
		for _, v := range h.Values(received) {
			httpHeaders = append(httpHeaders, Header{name: name, value: v.Value()})
		}
	}

	url := ""
	method := ""
	if tx.Type() == vsl.TxTypeRequest {
		method = tx.RecordValueByTag(tag.ReqMethod, received)
		url = tx.RecordValueByTag(tag.ReqURL, received)
	} else {
		method = tx.RecordValueByTag(tag.BereqMethod, received)
		url = tx.RecordValueByTag(tag.BereqURL, received)
	}

	return &HTTPRequest{
		method:  method,
		host:    host,
		url:     url,
		headers: httpHeaders,
	}
}
