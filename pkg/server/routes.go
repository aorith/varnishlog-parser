package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/render"
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

		connectTo := r.Form.Get("connectTo")
		switch connectTo {
		case "custom":
			connectTo = r.Form.Get("custom")
		case "backend":
			connectTo = r.Form.Get("transactionBackend")
		}

		f := content.ReqBuilderForm{
			Scheme:    r.Form.Get("scheme"),
			Received:  r.Form.Get("headers") == "received", // Use the received method/url and headers
			ConnectTo: connectTo,
		}

		err = content.ReqBuild(txsSet, f).Render(context.Background(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
	})

	mux.HandleFunc("POST /timestamps/{$}", func(w http.ResponseWriter, r *http.Request) {
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

		f := render.TimestampsForm{
			SinceLast:   r.Form.Get("timestampValue") == "last",
			Timeline:    r.Form.Get("timeline") == "on",
			Events:      r.Form["events"],
			OtherEvents: r.Form.Get("other-events") == "on",
		}

		err = content.RenderTimestampsTab(txsSet, f).Render(context.Background(), w)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
	})

	return mux
}
