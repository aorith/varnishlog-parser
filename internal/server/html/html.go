// SPDX-License-Identifier: MIT

package html

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/render"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/diagnostics"
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
		Sessions bool // include sessions
		Width    int  // timeline width
		Ticks    int  // number of ticks
	}
	Sequence render.SequenceConfig
}

// asHTML marks a render/helpers.go result as safe, pre-escaped HTML so
// html/template inserts it verbatim instead of escaping it.
// So it should be used only by functions that already HTML-escape its data.
func asHTML(s string) template.HTML {
	return template.HTML(s) // nolint:gosec
}

var funcMap = template.FuncMap{
	"headersView": func(ts vsl.TransactionSet, tx *vsl.Transaction) []template.HTML {
		lines := render.HTMLHeadersTable(ts, tx)
		htmlLines := make([]template.HTML, len(lines))

		for i, l := range lines {
			htmlLines[i] = asHTML(l)
		}

		return htmlLines
	},
	"renderTXLogTree": func(ts vsl.TransactionSet, tx *vsl.Transaction) template.HTML {
		return asHTML(render.TxTreeHTML(ts, tx))
	},
	"nonTransactionalLogTree": func(ts vsl.TransactionSet) template.HTML {
		return asHTML(render.NonTransactionalTreeHTML(ts))
	},
	"isTxTypeSession": func(tx *vsl.Transaction) bool { return tx.TXType == vsl.TxTypeSession },
	"curlCommand": func(tx *vsl.Transaction, cfg PageData) template.HTML {
		return asHTML(curlCommand(tx, cfg))
	},
	"hurlFile": func(tx *vsl.Transaction, cfg PageData) template.HTML {
		return asHTML(hurlFile(tx, cfg))
	},
	"timeline": func(ts vsl.TransactionSet, tx *vsl.Transaction, width, numTicks int) template.HTML {
		return asHTML(render.Timeline(ts, tx, width, numTicks))
	},
	"sequence": func(ts vsl.TransactionSet, tx *vsl.Transaction, cfg render.SequenceConfig) template.HTML {
		return asHTML(render.Sequence(ts, tx, cfg))
	},
	"timestampEventsSummary": summary.TimestampEventsSummary,
	"bandwidth":              summary.Bandwidth,
	"cacheStatus":            summary.CacheStatus,
	"diagnostics":            diagnostics.Run,
	"groupByRule":            diagnostics.GroupByRule,
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
	data.Views.Parse = "active"

	return executeTemplate(w, index, "main_layout.html", data)
}

func Parsed(w http.ResponseWriter, data PageData) error {
	parser := vsl.NewTransactionParser(strings.NewReader(data.Logs.Textinput))

	ts, err := parser.Parse()
	if err != nil {
		slog.Warn("failed to parse logs", "error", err)
		data.Error = err
		data.Views.Parse = "active"
	} else {
		slog.Info("txs", "count", len(ts.Transactions()))

		data.Transactions.Set = ts
		data.Transactions.Count = len(ts.Transactions())

		if data.Transactions.Count > 0 {
			data.Views.Overview = "active"
		} else {
			data.Views.Parse = "active"
		}

		data.Transactions.GroupCount = len(ts.GroupRelatedTransactions())
		data.Logs.Raw = ts.RawLog()
		data.Title = fmt.Sprintf("%d txs parsed", data.Transactions.Count)
	}

	return executeTemplate(w, parsed, "main_layout.html", data)
}

func Error(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)

	err2 := executeTemplate(w, errorTmpl, "main_layout.html", PageData{Title: "Error", Error: err})
	if err2 != nil {
		slog.Error("failed to render error template", "error", err2)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func PartialError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadRequest)

	err2 := executeTemplate(w, errorPartialTmpl, "error_partial.html", PageData{Title: "Error", Error: err})
	if err2 != nil {
		slog.Error("failed to render error partial template", "error", err2)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
// it avoids writing directly to 'w' to handle errors.
func executeTemplate(w io.Writer, tmpl *template.Template, name string, data any) error {
	buf := &bytes.Buffer{}

	err := tmpl.ExecuteTemplate(buf, name, data)
	if err == nil {
		_, err = buf.WriteTo(w)
		if err != nil {
			slog.Warn("failed to write template to response", "error", err)
		}
	}

	return err
}
