package chshare

import (
	"context"
	"net"
	"net/http"
	"sync"
)

//HTTPServer extends net/http Server and
//adds graceful shutdowns
type HTTPServer struct {
	*Logger
	*http.Server
	listener  net.Listener
	done      chan struct{}
	doneErr   error
	isStarted bool
	isShuttingDown bool
	stopper    sync.Once
}

//NewHTTPServer creates a new HTTPServer
func NewHTTPServer(logger *Logger) *HTTPServer {
	return &HTTPServer{
		Logger: logger.Fork("HTTPServer"),
		Server:   &http.Server{},
		listener: nil,
		done:  make(chan struct{}),
	}
}

// ListenAndServe Runs the HTTP server running in the background
// on the given bind address, invoking the provided handler for each
// request. It returns after the server has shutdown. The server can be
// shutdown either by cancelling the context or by calling shutdownWith().
func (h *HTTPServer) ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	if h.isStarted {
		return h.DebugErrorf("HTTP server has already been started")
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	h.Handler = handler
	h.listener = l
	h.isStarted = true
	go func() {
		h.ShutdownWith(h.Serve(l))
	}()
	go func() {
		select{
			case <-ctx.Done():
				h.ShutdownWith(ctx.Err())
			case <-h.done:
		}
	}()
	return h.Wait()
}

// ShutdownWith begins asynchronous shutdown of the server, with
// a preferred exit code. If shutdown has already begun, has no effect.
// Shutdown is complete when Wait() returns.
func (h *HTTPServer) ShutdownWith(err error) {
	h.stopper.Do(func(){
		go func(){
			h.isShuttingDown = true
			if h.isStarted {
				lerr := h.listener.Close()
				if lerr != nil {
					h.Debugf("HTTPserver: close of listener failed, ignoring: %s", lerr)
				}
			} else {
				h.isStarted = true
			}
			h.doneErr = err
			close(h.done)
		}()
	})
}

// Shutdown begins normal asynchronous shutdown of the server.
// If shutdown has already begun, has no effect.
// Shutdown is complete when Wait() returns.
func(h *HTTPServer) Shutdown() {
	h.ShutdownWith(nil)
}

// Close closes the HTTPServer and stops it from listening. Does not return until
// resources are freed.
func (h *HTTPServer) Close() error {
	h.Shutdown()
	return h.Wait()
}

// DoneChan returns a channel that is closed after the server has completely shut down
func (h *HTTPServer) DoneChan() chan struct{} {
	return h.done
}

// Wait waits for an HTTPServer to fully shut down. Returns final completion status.
func (h *HTTPServer) Wait() error {
	<-h.done
	return h.doneErr
}
