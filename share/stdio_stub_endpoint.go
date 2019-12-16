package chshare

import (
	"fmt"
	"os"
)

// StdioStubEndpoint implements a local Stdio stub
type StdioStubEndpoint struct {
	// Implements LocalStubChannelEndpoint
	BasicEndpoint
	pipeConn *PipeConn
}

// NewStdioStubEndpoint creates a new StdioStubEndpoint
func NewStdioStubEndpoint(
	logger *Logger,
	ced *ChannelEndpointDescriptor,
) (*StdioStubEndpoint, error) {
	myLogger := logger.Fork("StdioStubEndpoint")
	pipeConn, err := NewPipeConn(myLogger, os.Stdin, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to create stdio PipeConn: %s", myLogger.Prefix(), err)
	}
	ep := &StdioStubEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: myLogger,
			ced:    ced,
		},
		pipeConn: pipeConn,
	}
	return ep, nil
}

func (ep *StdioStubEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *StdioStubEndpoint) Close() error {
	ep.pipeConn.Close()
	return nil
}

// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to begin accepting Callers before you make the first Accept call. Part of
// AcceptorChannelEndpoint interface.
func (ep *StdioStubEndpoint) StartListening() error {
	return nil
}

// Accept listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration. This call does not return until a new connection is available or a
// error occurs. There is no way to cancel an Accept() request other than closing the endpoint. Part of
// the AcceptorChannelEndpoint interface.
func (ep *StdioStubEndpoint) Accept() (ChannelConn, error) {
	return ep.pipeConn, nil
}
