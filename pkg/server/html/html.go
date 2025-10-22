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
)

var funcMap = template.FuncMap{
	"uniqueRootParents": func(ts vsl.TransactionSet) []*vsl.Transaction { return ts.UniqueRootParents() },
	"mermaid":           func(ts vsl.TransactionSet, tx *vsl.Transaction) string { return render.SequenceDiagram(ts, tx) },
	"headersTableHTML": func(ts vsl.TransactionSet, tx *vsl.Transaction) []render.TableRow {
		return render.HeadersTableHTML(ts, tx)
	},
	"renderTXLogTree": func(ts vsl.TransactionSet, tx *vsl.Transaction) string { return render.TxTreeHTML(ts, tx) },
}

var (
	index = parseTemplate(
		"templates/layout/main_layout.html",
		"templates/content/index_content.html",
		"templates/partials/parse_form_partial.html",
		"templates/partials/parse_logo_partial.html",
		"templates/views/parse_view.html",
		"templates/unparsed.html",
	)
	parsed = parseTemplate(
		"templates/layout/main_layout.html",
		"templates/content/parsed_content.html",
		"templates/partials/parse_form_partial.html",
		"templates/partials/parse_logo_partial.html",
		"templates/views/*.html",
	)
	errorTmpl = parseTemplate("templates/layout/main_layout.html", "templates/error.html")
)

type PageData struct {
	Title   string
	Version string
	Error   error
	Logs    struct {
		Textinput string
		Raw       string
	}
	Transactions struct {
		Set        vsl.TransactionSet
		Count      int
		GroupCount int
	}
}

func Index(w http.ResponseWriter, data PageData) error {
	return executeTemplate(w, index, "main_layout.html", data)
}

func Parsed(w http.ResponseWriter, data PageData) error {
	parser := vsl.NewTransactionParser(strings.NewReader(data.Logs.Textinput))
	txs, err := parser.Parse()
	slog.Info("txs", "count", len(txs.Transactions()))
	if err != nil {
		slog.Warn("failed to parse logs", "error", err)
		return err
	}

	data.Transactions.Set = txs
	data.Transactions.Count = len(txs.Transactions())
	data.Transactions.GroupCount = len(txs.GroupRelatedTransactions())
	data.Logs.Raw = txs.RawLog()
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
