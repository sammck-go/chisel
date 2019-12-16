package chshare

import (
	"context"
	"fmt"
	"os"
)

// StdioSkeletonEndpoint implements a local Stdio skeleton
type StdioSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	pipeConn *PipeConn
}

// NewStdioSkeletonEndpoint creates a new StdioSkeletonEndpoint
func NewStdioSkeletonEndpoint(
	logger *Logger,
	ced *ChannelEndpointDescriptor,
) (*StdioSkeletonEndpoint, error) {
	myLogger := logger.Fork("StdioSkeletonEndpoint")
	pipeConn, err := NewPipeConn(myLogger, os.Stdin, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to create stdio PipeConn: %s", myLogger.Prefix(), err)
	}
	ep := &StdioSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: myLogger,
			ced:    ced,
		},
		pipeConn: pipeConn,
	}
	return ep, nil
}

func (ep *StdioSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *StdioSkeletonEndpoint) Close() error {
	ep.pipeConn.Close()
	return nil
}

// DialContext initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *StdioSkeletonEndpoint) DialContext(ctx context.Context, extraData []byte) (ChannelConn, error) {
	return ep.pipeConn, nil
}
