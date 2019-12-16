package chshare

import (
	"context"
	"fmt"
	"github.com/prep/socketpair"
)

// LoopSkeletonEndpoint implements a local Loop skeleton
type LoopSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	LoopServer *Loop5.Server
}

// NewLoopSkeletonEndpoint creates a new LoopSkeletonEndpoint
func NewLoopSkeletonEndpoint(
	logger *Logger,
	ced *ChannelEndpointDescriptor,
	LoopServer *Loop5.Server,
) (*LoopSkeletonEndpoint, error) {
	ep := &LoopSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("LoopSkeletonEndpoint: %s", ced),
			ced:    ced,
		},
	}
	return ep, nil
}

func (ep *LoopSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *LoopSkeletonEndpoint) Close() error {
	ep.lock.Lock()
	if !ep.closed {
		ep.closed = true
	}
	ep.lock.Unlock()
	return nil
}

// DialContext initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *LoopSkeletonEndpoint) DialContext(ctx context.Context, extraData []byte) (ChannelConn, error) {
	var err error
	ep.lock.Lock()
	if ep.closed {
		err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
	}
	ep.lock.Unlock()
	if err != nil {
		return nil, err
	}

	// Create a socket pair so that the Loop5 server has something to talk to and
	// we have something to return to the caller. This results in one hop through a socket
	// but it preserves our abstraction that requires endpoints to create their ChannelConn
	// first, then we wire them together with a pipe task.
	netConn, LoopNetConn, err := socketpair.New("unix")
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create socketpair: %s", ep.Logger.Prefix(), err)
	}

	// Now we can create a ChannelCon for our end of the connection
	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		netConn.Close()
		LoopNetConn.Close()
		return nil, fmt.Errorf("%s: Unable to wrap net.Conn with SocketConn: %s", ep.Logger.Prefix(), err)
	}

	err = ep.LoopServer.ServeConn(LoopNetConn)
	if err != nil {
		LoopNetConn.Close()
		conn.Close()
		return nil, fmt.Errorf("%s: Loop5 server refused connect: %s", ep.Logger.Prefix(), err)
	}

	return conn, nil
}
