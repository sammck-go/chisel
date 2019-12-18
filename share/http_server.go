package chshare

import (
	"errors"
	"net"
	"net/http"
	"sync"
)

//HTTPServer extends net/http Server and
//adds graceful shutdowns
type HTTPServer struct {
	*http.Server
	listener  net.Listener
	running   chan error
	isRunning bool
	closer    sync.Once
}

//NewHTTPServer creates a new HTTPServer
func NewHTTPServer() *HTTPServer {
	return &HTTPServer{
		Server:   &http.Server{},
		listener: nil,
		running:  make(chan error, 1),
	}
}

// GoListenAndServe starts an HTTP server running in the background
// on the given bind address, invoking the provided handler for each
// request.
func (h *HTTPServer) GoListenAndServe(addr string, handler http.Handler) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	h.isRunning = true
	h.Handler = handler
	h.listener = l
	go func() {
		h.closeWith(h.Serve(l))
	}()
	return nil
}

func (h *HTTPServer) closeWith(err error) {
	if !h.isRunning {
		return
	}
	h.isRunning = false
	h.running <- err
}

// Close closes the HTTPServer and stops it from listening
func (h *HTTPServer) Close() error {
	h.closeWith(nil)
	return h.listener.Close()
}

// Wait waits for an HTTPServer to fully shut down
func (h *HTTPServer) Wait() error {
	if !h.isRunning {
		return errors.New("Already closed")
	}
	return <-h.running
}
