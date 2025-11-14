// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tags"
)

type HTTPRequest struct {
	method  string
	host    string
	port    string
	url     string
	headers []Header
}

func (r HTTPRequest) Headers() []Header {
	return r.headers
}

type Header struct {
	name  string
	value string
}

func (h Header) Name() string {
	return h.name
}

func (h Header) Value() string {
	return h.value
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
// If received is true, initial (received) headers are used; otherwise, headers after VCL processing.
// excludedHeaders can contain an slice of strings, each one must be a header name
func NewHTTPRequest(tx *vsl.Transaction, received bool, excludedHeaders []string) (*HTTPRequest, error) {
	if tx.TXType == vsl.TxTypeSession {
		return nil, fmt.Errorf("cannot create an http request from a transaction of type session")
	}

	headers := tx.ReqHeaders

	host := headers.Get("host", received)
	port := ""
	if strings.Contains(host, ":") {
		var err error
		host, port, err = ParseBackend(headers.Get("host", received))
		if err != nil {
			return nil, err
		}
	}

	// Ensure that excludeHeaders are in canonical format
	for i, n := range excludedHeaders {
		excludedHeaders[i] = vsl.CanonicalHeaderName(n)
	}

	httpHeaders := []Header{}
	for name, h := range headers {
		if name == vsl.HdrNameHost || slices.Contains(excludedHeaders, name) {
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
	if tx.TXType == vsl.TxTypeRequest {
		method = tx.RecordValueByTag(tags.ReqMethod, received)
		url = tx.RecordValueByTag(tags.ReqURL, received)
	} else {
		method = tx.RecordValueByTag(tags.BereqMethod, received)
		url = tx.RecordValueByTag(tags.BereqURL, received)
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
//
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
	hostURL = escapeDoubleQuotes(hostURL)

	// Initial command
	s.WriteString(fmt.Sprintf(`curl "%s%s%s"`+" \\\n", scheme, hostURL, escapeDoubleQuotes(r.url)))

	switch r.method {
	case "GET":
		// Default
	case "POST", "PUT", "PATCH":
		s.WriteString("    -X " + r.method + " \\\n")
		s.WriteString("    -d '<body-unavailable>' \\\n")
	case "HEAD":
		s.WriteString("    --head \\\n")
	default:
		s.WriteString("    -X " + r.method + " \\\n")
	}

	// Headers
	for _, h := range r.headers {
		if h.name == vsl.HdrNameHost {
			continue
		}
		s.WriteString(fmt.Sprintf(`    -H "%s: %s"`+" \\\n", escapeDoubleQuotes(h.name), escapeDoubleQuotes(h.value)))
	}

	// Default parameters
	s.WriteString("    -qsv")
	if scheme == "https://" {
		s.WriteString(" -k")
	}
	s.WriteString(" -o /dev/null")

	// Connect-to
	// --connect-to HOST1:PORT1:HOST2:PORT2
	// when you would connect to HOST1:PORT1, actually connect to HOST2:PORT2
	if backend != nil {
		s.WriteString(fmt.Sprintf(" \\\n    "+`--connect-to "%s:%s:%s"`, hostURL, escapeDoubleQuotes(backend.host), backend.port))
	}

	return s.String()
}

// HurlFile generates a new hurl file as a string
//
// scheme can be "auto", "http://" or "https://"
func (r *HTTPRequest) HurlFile(scheme string, backend *Backend) string {
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

	// Start hurl file
	s.WriteString(fmt.Sprintf("%s %s%s%s\n", r.method, scheme, hostURL, r.url))

	// Headers
	for _, h := range r.headers {
		if h.name == vsl.HdrNameHost {
			continue
		}
		s.WriteString(fmt.Sprintf("%s: %s\n", h.name, h.value))
	}

	// Options
	if scheme == "https://" {
		s.WriteString("\n[Options]\ninsecure: true\n")
	}

	// Connect-to
	// --connect-to HOST1:PORT1:HOST2:PORT2
	// when you would connect to HOST1:PORT1, actually connect to HOST2:PORT2
	if backend != nil {
		s.WriteString("\n# To connect to the backend run the hurl file as:\n")
		s.WriteString(fmt.Sprintf(`# hurl --connect-to "%s:%s:%s" file.hurl`,
			escapeDoubleQuotes(hostURL), escapeDoubleQuotes(backend.host), backend.port,
		))
	}

	return s.String()
}

// escapeDoubleQuotes is an utility function to escape double quotes ;)
func escapeDoubleQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
