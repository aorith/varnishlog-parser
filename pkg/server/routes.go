package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/server/templates/content"
	"github.com/aorith/varnishlog-parser/pkg/server/templates/pages"
	"github.com/aorith/varnishlog-parser/pkg/server/templates/partials"
	"github.com/aorith/varnishlog-parser/vsl"
)

func (s *vlogServer) registerRoutes() http.Handler {
	mux := http.NewServeMux()

	// Static
	mux.Handle("GET /static/", http.FileServerFS(assets.Assets))

	// Full pages
	mux.Handle("GET /{$}", templ.Handler(pages.Initial(s.version)))
	mux.HandleFunc("POST /{$}", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		p := vsl.NewTransactionParser(strings.NewReader(r.Form.Get("logs")))
		txsSet, err := p.Parse()
		if err != nil {
			err = pages.Error(s.version, err).Render(context.Background(), w)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, err)
				return
			}
			return
		}

		err = pages.Parsed(s.version, txsSet).Render(context.Background(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
	})

	// HTMX Partials
	mux.HandleFunc("POST /reqbuilder/{$}", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		p := vsl.NewTransactionParser(strings.NewReader(r.Form.Get("logs")))
		txsSet, err := p.Parse()
		if err != nil {
			err = partials.ErrorMsg(err).Render(context.Background(), w)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, err)
				return
			}
			return
		}

		var rt int
		rtValue := r.Form.Get("sendTo")
		switch rtValue {
		case "domain":
			rt = content.SendToDomain
		case "backend":
			rt = content.SendToBackend
		case "localhost":
			rt = content.SendToLocalhost
		case "custom":
			rt = content.SendToCustom
		}

		f := content.ReqBuilderForm{
			TXID:            r.Form.Get("transaction"),
			HTTPS:           r.Form.Get("https") == "on",
			OriginalHeaders: r.Form.Get("headerType") == "original",
			OriginalURL:     r.Form.Get("urlType") == "original",
			ResolveTo:       rt,
			CustomResolve:   r.Form.Get("customResolve"),
		}

		if f.ResolveTo == content.SendToBackend {
			f.CustomResolve = r.Form.Get("transactionBackend")
		}

		// Find the tx which should still be present in the "New Parse" textarea
		tx, ok := txsSet.TransactionsMap()[f.TXID]
		if !ok {
			err = partials.ErrorMsg(fmt.Errorf(`Transaction %q not found. Did you reset the "New Parse" textarea?`, f.TXID)).Render(context.Background(), w)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, err)
				return
			}
			return
		}

		err = content.ReqBuild(txsSet, tx, f).Render(context.Background(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
	})

	return mux
}
