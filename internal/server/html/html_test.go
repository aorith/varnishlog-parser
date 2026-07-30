// SPDX-License-Identifier: MIT

package html

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndexRenders(t *testing.T) {
	w := httptest.NewRecorder()

	err := Index(w, PageData{Version: "test"})
	if err != nil {
		t.Fatalf("Index() failed: %s", err)
	}

	body := w.Body.String()

	for _, want := range []string{`class="nav-link"`, `/static/tabs.js`, `class="view active" id="parse-view"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Index() output missing %q", want)
		}
	}
}

func TestParsedRenders(t *testing.T) {
	w := httptest.NewRecorder()

	data := PageData{Version: "test"}
	data.Logs.Textinput = `*   << Session  >> 1
-   Begin          sess 0 HTTP/1
-   SessOpen       192.168.50.1 55650 http 192.168.50.10 80 1728889150.256391 26
-   Link           req 2 rxreq
-   SessClose      REM_CLOSE 0.003
-   End
**  << Request  >> 2
--  Begin          req 1 rxreq
--  ReqMethod      GET
--  ReqURL         /
--  End
`

	err := Parsed(w, data)
	if err != nil {
		t.Fatalf("Parsed() failed: %s", err)
	}

	body := w.Body.String()

	for _, want := range []string{`class="view active" id="overview-view"`, `id="tx-2-req-rxreq"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Parsed() output missing %q", want)
		}
	}

	if strings.Contains(body, `class="view active" id="parse-view"`) {
		t.Error("Parsed() output: parse-view should not be active once a transaction was parsed")
	}
}

// TestParsedEscapesSpecialCharacters guards against a bug where VSL data
// containing HTML-special characters (an unknown tag's raw value, or a
// VCL_Log message) broke the raw log <pre> block and the LogTree view.
func TestParsedEscapesSpecialCharacters(t *testing.T) {
	w := httptest.NewRecorder()

	data := PageData{Version: "test"}
	data.Logs.Textinput = `*** << BeReq    >> 3
--- Begin          bereq 1 fetch
--- VCL_Log        xbody.regsub() '<Location>http://example.com/'
--- XBody          XBODY_REGSUB_<Locatio-0 0
--- End
`

	err := Parsed(w, data)
	if err != nil {
		t.Fatalf("Parsed() failed: %s", err)
	}

	body := w.Body.String()

	if strings.Contains(body, "<Location>") || strings.Contains(body, "<Locatio-0") {
		t.Errorf("Parsed() output contains unescaped HTML-special characters from VSL data:\n%s", body)
	}

	for _, want := range []string{"&lt;Location&gt;", "XBODY_REGSUB_&lt;Locatio-0"} {
		if !strings.Contains(body, want) {
			t.Errorf("Parsed() output missing escaped value %q", want)
		}
	}
}

func TestParsedRendersErrorFallback(t *testing.T) {
	w := httptest.NewRecorder()

	data := PageData{Version: "test"}
	data.Logs.Textinput = "not a valid varnishlog"

	err := Parsed(w, data)
	if err != nil {
		t.Fatalf("Parsed() failed: %s", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, `class="view active" id="parse-view"`) {
		t.Error("Parsed() output: parse-view should be active on a parse error")
	}
}
