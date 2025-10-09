package render

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

type HTTPRequest struct {
	method  string
	host    string
	port    string
	url     string
	headers []Header
}

type Header struct {
	name  string
	value string
}

type Backend struct {
	host string
	port string
}

func NewBackend(host string, port string) *Backend {
	return &Backend{host: host, port: port}
}

// NewHTTPRequest constructs an HTTPRequest from a Varnish transaction.
// Returns nil if the transaction type is session.
//
// If fromResponse is true, fields are extracted from the response; otherwise, from the request.
// If received is true, initial (received) headers are used; otherwise, headers after VCL processing.
func NewHTTPRequest(tx *vsl.Transaction, fromResponse, received bool) (*HTTPRequest, error) {
	if tx.Type() == vsl.TxTypeSession {
		return nil, fmt.Errorf("cannot create an http request from a transaction of type session")
	}

	var headers vsl.Headers
	if fromResponse {
		headers = tx.RespHeaders()
	} else {
		headers = tx.ReqHeaders()
	}

	host := headers.Get("host", received)
	port := ""
	if strings.Contains(host, ":") {
		var err error
		host, port, err = ParseBackend(headers.Get("host", received))
		if err != nil {
			return nil, err
		}
	}

	httpHeaders := []Header{}
	for name, h := range headers {
		if name == vsl.HdrNameHost {
			continue
		}
		for _, v := range h.Values(received) {
			if v.State() == vsl.HdrStateDeleted {
				continue
			}
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

	sort.Slice(httpHeaders, func(i, j int) bool {
		return httpHeaders[i].name < httpHeaders[j].name
	})

	return &HTTPRequest{
		method:  method,
		host:    host,
		port:    port,
		url:     url,
		headers: httpHeaders,
	}, nil
}

// CurlCommand generates a new curl command as a string
// scheme can be "auto", "http://" or "https://"
func (r *HTTPRequest) CurlCommand(scheme string, backend *Backend) string {
	var s strings.Builder

	// Parse scheme
	switch scheme {
	case "auto":
		if r.port == "443" {
			scheme = "https://"
		} else {
			// default to http for 80, empty, or any other port
			scheme = "http://"
		}
	case "http://", "https://":
		// keep as-is
	default:
		return "invalid scheme: " + scheme
	}

	// Build host URL, append port only when provided
	hostURL := r.host
	if r.port != "" {
		hostURL = net.JoinHostPort(r.host, r.port)
	}

	// Initial command
	s.WriteString(fmt.Sprintf(`curl "%s%s%s"`+" \\\n", scheme, hostURL, r.url))

	switch r.method {
	case "POST", "PUT", "PATCH":
		s.WriteString("    -X " + r.method + " \\\n")
		s.WriteString("    -d '<body-unavailable>' \\\n")
	default:
		s.WriteString("    -X " + r.method + " \\\n")
	}

	// Headers
	for _, h := range r.headers {
		if h.name == vsl.HdrNameHost {
			continue
		}
		hdrVal := strings.ReplaceAll(h.value, `"`, `\"`)
		s.WriteString(fmt.Sprintf(`    -H "%s: %s"`+" \\\n", h.name, hdrVal))
	}

	// Default parameters
	s.WriteString("    -qsv -k -o /dev/null")

	// Connect-to
	// --connect-to HOST1:PORT1:HOST2:PORT2
	// when you would connect to HOST1:PORT1, actually connect to HOST2:PORT2
	if backend != nil {
		s.WriteString(fmt.Sprintf(" \\\n    "+`--connect-to "%s:%s:%s"`, hostURL, backend.host, backend.port))
	}

	return s.String()
}
