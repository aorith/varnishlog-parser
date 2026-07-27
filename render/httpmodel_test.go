// SPDX-License-Identifier: MIT

package render

import (
	"os"
	"os/exec"
	"strings"
	"testing"
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
