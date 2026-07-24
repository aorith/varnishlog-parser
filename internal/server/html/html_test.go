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
