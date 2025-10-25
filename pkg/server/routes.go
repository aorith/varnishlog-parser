// SPDX-License-Identifier: MIT

package server

import (
	"log/slog"
	"net/http"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/server/html"
)

func indexHandler(version string) func(http.ResponseWriter, *http.Request) {
	data := html.PageData{Version: version}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := html.Index(w, data); err != nil {
			slog.Warn("failed to render template", "error", err)
			html.Error(w, err)
		}
	}
}

func parseHandler(version string) func(http.ResponseWriter, *http.Request) {
	data := html.PageData{Version: version}

	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.Error(w, err)
			return
		}

		switch r.Form.Get("action") {
		case "eg-simple":
			data.Logs.Textinput = assets.VCLMissingChild1
		case "eg-complex":
			data.Logs.Textinput = assets.VCLComplete1
		default:
			data.Logs.Textinput = r.Form.Get("logs")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := html.Parsed(w, data); err != nil {
			slog.Warn("failed to render template", "error", err)
			html.Error(w, err)
		}
	}
}

func (s *vlogServer) registerRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(assets.Assets))
	mux.HandleFunc("GET /static/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css; charset=utf-8")
		w.Write(assets.CombinedCSS)
	})

	mux.HandleFunc("GET /{$}", indexHandler(s.version))
	mux.HandleFunc("POST /{$}", parseHandler(s.version))

	return mux
}
