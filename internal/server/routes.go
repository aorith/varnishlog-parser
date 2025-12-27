// SPDX-License-Identifier: MIT

package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/internal/server/html"
)

func indexHandler(version string) func(http.ResponseWriter, *http.Request) {
	data := html.PageData{Version: version}

	// Default values
	data.Sequence.Distance = 350
	data.Sequence.StepHeight = 40
	data.Sequence.IncludeCalls = false
	data.Sequence.IncludeReturns = false
	data.Sequence.IncludeVCLLogs = false
	data.Sequence.TrackURLAndHost = false

	data.Timeline.Sessions = false
	data.Timeline.Precision = 1200
	data.Timeline.Ticks = 10

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
		// Check if an example btn was pressed or it was the regular parse btn
		case "eg-simple":
			data.Logs.Textinput = assets.VCLSimplePOST
		case "eg-cached":
			data.Logs.Textinput = assets.VCLCached
		case "eg-streaming-hit":
			data.Logs.Textinput = assets.VCLStreamingHit
		case "eg-esi1":
			data.Logs.Textinput = assets.VCLESI1
		case "eg-req-restart":
			data.Logs.Textinput = assets.VCLRestart
		case "eg-esi-synth":
			data.Logs.Textinput = assets.VCLESISynth
		default:
			data.Logs.Textinput = r.Form.Get("logs")
		}

		// Sequence settings
		distance, err := strconv.Atoi(r.Form.Get("distance"))
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.PartialError(w, err)
			return
		}
		data.Sequence.Distance = distance

		stepHeight, err := strconv.Atoi(r.Form.Get("stepHeight"))
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.PartialError(w, err)
			return
		}
		data.Sequence.StepHeight = stepHeight

		data.Sequence.IncludeCalls = r.Form.Get("includeCalls") == "yes"
		data.Sequence.IncludeReturns = r.Form.Get("includeReturns") == "yes"
		data.Sequence.IncludeVCLLogs = r.Form.Get("includeVCLLogs") == "yes"
		data.Sequence.TrackURLAndHost = r.Form.Get("trackURLAndHost") == "yes"

		// Timeline settings
		data.Timeline.Sessions = r.Form.Get("sessions") == "yes"
		precision, err := strconv.Atoi(r.Form.Get("precision"))
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.PartialError(w, err)
			return
		}
		data.Timeline.Precision = precision
		numTicks, err := strconv.Atoi(r.Form.Get("ticks"))
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.PartialError(w, err)
			return
		}
		data.Timeline.Ticks = numTicks

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := html.Parsed(w, data); err != nil {
			slog.Warn("failed to render template", "error", err)
			html.Error(w, err)
		}
	}
}

func reqBuilderHandler(version string) func(http.ResponseWriter, *http.Request) {
	data := html.PageData{Version: version}

	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			slog.Warn("failed to parse form", "error", err)
			html.PartialError(w, err)
			return
		}

		data.Logs.Textinput = r.Form.Get("logs")
		data.ReqBuild.Scheme = r.Form.Get("scheme")
		data.ReqBuild.ReceivedHeaders = r.Form.Get("headers") == "received"
		data.ReqBuild.ExcludedHeaders = r.Form.Get("excluded")
		data.ReqBuild.ConnectTo = r.Form.Get("connectTo")
		data.ReqBuild.Backend = r.Form.Get("backend")
		data.ReqBuild.ConnectCustom = r.Form.Get("custom")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := html.ReqBuild(w, data); err != nil {
			slog.Warn("failed to render template", "error", err)
			html.PartialError(w, err)
			return
		}
	}
}

func (s *vlogServer) registerRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(assets.Assets))
	mux.HandleFunc("GET /static/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css; charset=utf-8")
		_, err := w.Write(assets.CombinedCSS)
		if err != nil {
			panic(err)
		}
	})

	mux.HandleFunc("GET /{$}", indexHandler(s.version))
	mux.HandleFunc("POST /{$}", parseHandler(s.version))
	mux.HandleFunc("POST /reqbuilder/{$}", reqBuilderHandler(s.version))

	return mux
}
