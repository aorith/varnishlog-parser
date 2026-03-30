// SPDX-License-Identifier: MIT

package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type vlogServer struct {
	bind    string
	port    int
	version string
}

func StartServer(bind string, port int, version string) error {
	srv := newServer(bind, port, version)

	err := srv.ListenAndServe()
	if err != nil {
		slog.Error("Server error", "error", err)

		return err
	}

	return nil
}

func newServer(bind string, port int, version string) *http.Server {
	srv := &vlogServer{
		bind:    bind,
		port:    port,
		version: version,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", srv.bind, srv.port),
		Handler:      srv.registerRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	slog.Info("Listening", "address", srv.bind, "port", srv.port)

	return server
}
