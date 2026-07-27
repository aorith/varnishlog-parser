// SPDX-License-Identifier: MIT

package render

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
)

func TestShellSingleQuote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "''"},
		{"plain", "'plain'"},
		{"it's here", `'it'\''s here'`},
		{"$(touch pwned)", "'$(touch pwned)'"},
	}

	for _, test := range tests {
		if got := shellSingleQuote(test.in); got != test.want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// TestCurlCommandIsShellSafe verifies header values coming from untrusted
// HTTP traffic (e.g. a crafted User-Agent) can't break out of the generated
// curl command's quoting and execute when a user copy-pastes it into a
// shell.
func TestCurlCommandIsShellSafe(t *testing.T) {
	dir := t.TempDir()
	marker := dir + "/marker"
	argsFile := dir + "/args"

	req := &HTTPRequest{
		method: "GET",
		host:   "example.com",
		url:    "/path",
		headers: []Header{
			{name: "User-Agent", value: "normal$(touch " + marker + ")`id`"},
			{name: "X-Quote", value: "value's got a quote"},
		},
	}

	cmd := req.CurlCommand("http://", nil)

	// Swap the leading "curl" for a shell function that just records its
	// arguments, so we can inspect exactly what bash parsed/expanded
	// without making a real network call.
	script := "capture() { printf '%s\\n' \"$@\" > " + argsFile + "; }\ncapture" + strings.TrimPrefix(cmd, "curl")

	err := exec.Command("bash", "-c", script).Run() // nolint:gosec,noctx
	if err != nil {
		t.Fatalf("bash failed to run the generated command: %s", err)
	}

	_, err = os.Stat(marker)
	if err == nil {
		t.Fatal("shell metacharacters in a header value were executed by the generated curl command")
	}

	out, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading captured args: %s", err)
	}

	if !strings.Contains(string(out), "User-Agent: normal$(touch "+marker+")`id`") {
		t.Errorf("malicious header value was not preserved literally, got:\n%s", out)
	}

	if !strings.Contains(string(out), "X-Quote: value's got a quote") {
		t.Errorf("header value with a single quote was not preserved literally, got:\n%s", out)
	}
}

// TestNewHTTPRequestOmitsStaleBodyFramingHeaders checks that Content-Length
// and Transfer-Encoding aren't forwarded for methods whose body varnishlog
// never records (POST/PUT/PATCH) - since CurlCommand/HurlFile substitute a
// placeholder body, those headers would describe a body that no longer
// matches and would break the request on the wire.
func TestNewHTTPRequestOmitsStaleBodyFramingHeaders(t *testing.T) {
	// simple-post.txt is a POST with a real Content-Length; add a
	// Transfer-Encoding header too so both get exercised.
	logText := strings.Replace(
		assets.VCLSimplePOST,
		"--  ReqHeader      Content-Length: 129\n",
		"--  ReqHeader      Content-Length: 129\n--  ReqHeader      Transfer-Encoding: chunked\n",
		1,
	)

	p := vsl.NewTransactionParser(strings.NewReader(logText))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	var reqTx *vsl.Transaction

	for _, tx := range ts.Transactions() {
		if tx.TXType == vsl.TxTypeRequest {
			reqTx = tx

			break
		}
	}

	if reqTx == nil {
		t.Fatal("no Request transaction found in the test log")
	}

	httpReq, err := NewHTTPRequest(reqTx, true, nil)
	if err != nil {
		t.Fatalf("NewHTTPRequest() failed: %s", err)
	}

	for _, h := range httpReq.Headers() {
		if h.Name() == vsl.HdrNameContentLength {
			t.Errorf("Content-Length should be excluded for POST (placeholder body), got value %q", h.Value())
		}

		if h.Name() == vsl.HdrNameTransferEncoding {
			t.Errorf("Transfer-Encoding should be excluded for POST (placeholder body), got value %q", h.Value())
		}
	}

	cmd := httpReq.CurlCommand("http://", nil)
	if strings.Contains(cmd, "Content-Length") || strings.Contains(cmd, "Transfer-Encoding") {
		t.Errorf("generated curl command still references stale body-framing headers:\n%s", cmd)
	}
}
