// SPDX-License-Identifier: MIT

package html

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"text/template"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/render"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/summary"
)

type PageData struct {
	Title   string
	Version string
	Error   error
	Views   struct {
		Parse    string
		Overview string
	}
	Logs struct {
		Textinput string
		Raw       string
	}
	Transactions struct {
		Set        vsl.TransactionSet
		Count      int
		GroupCount int
	}
	ReqBuild struct {
		Scheme          string // auto, http://, https://
		ReceivedHeaders bool
		ExcludedHeaders string
		ConnectTo       string // backend, custom
		Backend         string // auto, none, <host:port>
		ConnectCustom   string // <host:port>
	}
	Timeline struct {
		Sessions  bool // include sessions
		Precision int  // timeline precision
		Ticks     int  // number of ticks
	}
	Sequence render.SequenceConfig
}

var funcMap = template.FuncMap{
	"headersView":            render.HeadersView,
	"renderTXLogTree":        render.TxTreeHTML,
	"isTxTypeSession":        func(tx *vsl.Transaction) bool { return tx.TXType == vsl.TxTypeSession },
	"curlCommand":            curlCommand,
	"timeline":               render.Timeline,
	"sequence":               render.Sequence,
	"timestampEventsSummary": summary.TimestampEventsSummary,
}

var (
	index = parseTemplate(
		"templates/layout/main_layout.html",
		"templates/content/index_content.html",
		"templates/partials/parse_form_partial.html",
		"templates/views/parse_view.html",
		"templates/unparsed.html",
	)
	parsed = parseTemplate(
		"templates/layout/main_layout.html",
		"templates/content/parsed_content.html",
		"templates/partials/parse_form_partial.html",
		"templates/views/*.html",
	)
	errorTmpl        = parseTemplate("templates/layout/main_layout.html", "templates/error.html")
	errorPartialTmpl = parseTemplate("templates/partials/error_partial.html")

	reqBuildPartial = parseTemplate("templates/partials/reqbuild_partial.html")
)

func Index(w http.ResponseWriter, data PageData) error {
	data.Views.Parse = "checked"
	return executeTemplate(w, index, "main_layout.html", data)
}

func Parsed(w http.ResponseWriter, data PageData) error {
	parser := vsl.NewTransactionParser(strings.NewReader(data.Logs.Textinput))
	ts, err := parser.Parse()
	slog.Info("txs", "count", len(ts.Transactions()))
	if err != nil {
		slog.Warn("failed to parse logs", "error", err)
		return err
	}

	data.Transactions.Set = ts
	data.Transactions.Count = len(ts.Transactions())
	if data.Transactions.Count > 0 {
		data.Views.Overview = "checked"
	} else {
		data.Views.Parse = "checked"
	}
	data.Transactions.GroupCount = len(ts.GroupRelatedTransactions())
	data.Logs.Raw = ts.RawLog()

	return executeTemplate(w, parsed, "main_layout.html", data)
}

func Error(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	err2 := executeTemplate(w, errorTmpl, "main_layout.html", PageData{Title: "Error", Error: err})
	if err2 != nil {
		panic(err2)
	}
}

func PartialError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	err2 := executeTemplate(w, errorPartialTmpl, "error_partial.html", PageData{Title: "Error", Error: err})
	if err2 != nil {
		panic(err2)
	}
}

func ReqBuild(w http.ResponseWriter, data PageData) error {
	parser := vsl.NewTransactionParser(strings.NewReader(data.Logs.Textinput))
	ts, err := parser.Parse()
	if err != nil {
		slog.Warn("failed to parse logs", "error", err)
		return err
	}
	data.Transactions.Set = ts
	return executeTemplate(w, reqBuildPartial, "reqbuild_partial.html", data)
}

func parseTemplate(files ...string) *template.Template {
	return template.Must(template.New("").Funcs(funcMap).ParseFS(assets.Templates, files...))
}

// executeTemplate is a wrapper around *template.Template
// it avoids writing directly to 'w' to handle errors
func executeTemplate(w io.Writer, tmpl *template.Template, name string, data any) error {
	buf := &bytes.Buffer{}
	err := tmpl.ExecuteTemplate(buf, name, data)
	if err == nil {
		_, err = buf.WriteTo(w)
		if err != nil {
			panic(err)
		}
	}
	return err
}

func curlCommand(tx *vsl.Transaction, scheme, connectTo, backendStr, custom string, received bool, excluded string) string {
	var backend *render.Backend

	switch connectTo {
	case "backend":
		switch backendStr {
		case "none":
			backend = nil

		case "auto":
			if tx.TXType == vsl.TxTypeBereq {
				host, port, err := render.ParseBackend(tx.GetBackendConnStr())
				if err != nil {
					return "error parsing backend: " + err.Error()
				}
				backend = render.NewBackend(host, port)
			}

		default:
			host, port, err := render.ParseBackend(backendStr)
			if err != nil {
				return "error parsing backend: " + err.Error()
			}
			backend = render.NewBackend(host, port)
		}

	case "custom":
		if custom == "" {
			return "no value provided for the custom backend"
		}
		host, port, err := render.ParseBackend(custom)
		if err != nil {
			return "error parsing backend: " + err.Error()
		}
		backend = render.NewBackend(host, port)

	default:
		return "unknown value for connectTo: " + connectTo
	}

	excludedHeaders := strings.Split(excluded, ",")
	for i, n := range excludedHeaders {
		excludedHeaders[i] = strings.TrimSpace(n)
	}

	httpReq, err := render.NewHTTPRequest(tx, received, excludedHeaders)
	if err != nil {
		return "error generating curl command: " + err.Error()
	}

	return httpReq.CurlCommand(scheme, backend)
}
