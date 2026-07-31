// SPDX-License-Identifier: MIT

package render

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"slices"
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

// NewHTTPRequest constructs an HTTPRequest from a Varnish transaction.
// Returns nil if the transaction type is session.
//
// If received is true, initial (received) headers are used; otherwise, headers after VCL processing.
// excludedHeaders can contain an slice of strings, each one must be a header name.
func NewHTTPRequest(tx *vsl.Transaction, received bool, excludedHeaders []string) (*HTTPRequest, error) {
	if tx.TXType == vsl.TxTypeSession {
		return nil, errors.New("cannot create an http request from a transaction of type session")
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

	var url, method string
	if tx.TXType == vsl.TxTypeRequest {
		method = tx.RecordValueByTag(tags.ReqMethod, received)
		url = tx.RecordValueByTag(tags.ReqURL, received)
	} else {
		method = tx.RecordValueByTag(tags.BereqMethod, received)
		url = tx.RecordValueByTag(tags.BereqURL, received)
	}

	// Canonicalize excludedHeaders into a copy
	canonicalExcluded := make([]string, len(excludedHeaders))
	for i, n := range excludedHeaders {
		canonicalExcluded[i] = vsl.CanonicalHeaderName(n)
	}

	noBody := methodHasNoRecordedBody(method)

	httpHeaders := []Header{}

	for name, h := range headers {
		if name == vsl.HdrNameHost || slices.Contains(canonicalExcluded, name) {
			continue
		}

		// A placeholder body is used for these methods (see CurlCommand/
		// HurlFile), so headers describing the real body's framing would
		// no longer match and must not be forwarded.
		if noBody && (name == vsl.HdrNameContentLength || name == vsl.HdrNameTransferEncoding) {
			continue
		}

		for _, v := range h.Values(received) {
			if v.State() == vsl.HdrStateDeleted {
				continue
			}

			httpHeaders = append(httpHeaders, Header{name: name, value: v.Value()})
		}
	}

	slices.SortFunc(httpHeaders, func(a, b Header) int {
		return cmp.Compare(a.name, b.name)
	})

	return &HTTPRequest{
		method:  method,
		host:    host,
		port:    port,
		url:     url,
		headers: httpHeaders,
	}, nil
}

func (r *HTTPRequest) Headers() []Header {
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

	// Initial command
	fmt.Fprintf(&s, "curl %s"+" \\\n", shellSingleQuote(scheme+hostURL+r.url)) //nolint:revive

	switch r.method {
	case "GET":
		// Default
	case "HEAD":
		s.WriteString("    --head \\\n") //nolint:revive
	default:
		s.WriteString("    -X " + r.method + " \\\n") //nolint:revive
	}

	// Headers
	for _, h := range r.headers {
		fmt.Fprintf(&s, "    -H %s"+" \\\n", shellSingleQuote(h.name+": "+h.value)) //nolint:revive
	}

	// Body
	if methodHasNoRecordedBody(r.method) {
		s.WriteString("    -d '<body-unavailable>' \\\n") //nolint:revive
	}

	// Default parameters
	s.WriteString("    -qsv") //nolint:revive

	if scheme == "https://" {
		s.WriteString(" -k") //nolint:revive
	}

	s.WriteString(" -o /dev/null") //nolint:revive

	// Connect-to
	// --connect-to HOST1:PORT1:HOST2:PORT2
	// when you would connect to HOST1:PORT1, actually connect to HOST2:PORT2
	if backend != nil {
		fmt.Fprintf(&s, " \\\n    --connect-to %s", shellSingleQuote("::"+backend.host+":"+backend.port)) //nolint:revive
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
	fmt.Fprintf(&s, "%s %s%s%s\n", r.method, scheme, hostURL, r.url) //nolint:revive

	// Headers
	for _, h := range r.headers {
		fmt.Fprintf(&s, "%s: %s\n", h.name, h.value) //nolint:revive
	}

	// Options
	if scheme == "https://" {
		s.WriteString("\n[Options]\ninsecure: true\n") //nolint:revive
	}

	// Body
	if methodHasNoRecordedBody(r.method) {
		s.WriteString("\n# Body is not available within varnishlog, add it manually.\n") //nolint:revive
	}

	// Connect-to
	// --connect-to HOST1:PORT1:HOST2:PORT2
	// when you would connect to HOST1:PORT1, actually connect to HOST2:PORT2
	if backend != nil {
		s.WriteString("\n# To connect to the backend run the hurl file as:\n")                                    //nolint:revive
		fmt.Fprintf(&s, "# hurl --connect-to %s file.hurl", shellSingleQuote("::"+backend.host+":"+backend.port)) //nolint:revive
	}

	return s.String()
}

// methodHasNoRecordedBody reports whether varnishlog doesn't capture a
// request body for this method, meaning the generated request must fall
// back to a placeholder body.
func methodHasNoRecordedBody(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}

// shellSingleQuote quotes s as a single POSIX shell argument. Header values
// and URLs come from parsed HTTP traffic, which cannot be trusted (e.g. an
// attacker-controlled User-Agent or Referer) - single quotes suppress all
// shell expansion ($, `, \, !), unlike double quotes, which is what keeps a
// copy-pasted command from executing anything embedded in that data.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
