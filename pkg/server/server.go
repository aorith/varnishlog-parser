package server

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type vlogServer struct {
	bind    string
	port    int
	version string
}

func StartServer(bind string, port int, version string) {
	srv := newServer(bind, port, version)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
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

	log.Printf("Listening on %s:%d\n", srv.bind, srv.port)

	return server
}
