// SPDX-License-Identifier: MIT

package server

import (
	"net/http"

	"github.com/aorith/varnishlog-parser/assets"
)

func (s *vlogServer) registerRoutes() http.Handler {
	mux := http.NewServeMux()

	// Static
	mux.Handle("GET /static/", http.FileServerFS(assets.Assets))

	return mux
}
