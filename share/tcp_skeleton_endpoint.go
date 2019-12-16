package chshare

import (
	"context"
	"fmt"
	"net"
)

// TCPSkeletonEndpoint implements a local TCP skeleton
type TCPSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
}

// NewTCPSkeletonEndpoint creates a new TCPSkeletonEndpoint
func NewTCPSkeletonEndpoint(logger *Logger, ced *ChannelEndpointDescriptor) (*TCPSkeletonEndpoint, error) {
	ep := &TCPSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("TCPSkeletonEndpoint: %s", ced),
			ced:    ced,
		},
	}
	return ep, nil
}

func (ep *TCPSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *TCPSkeletonEndpoint) Close() error {
	ep.lock.Lock()
	if !ep.closed {
		ep.closed = true
	}
	ep.lock.Unlock()
	return nil
}

// DialContext initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *TCPSkeletonEndpoint) DialContext(ctx context.Context, extraData []byte) (ChannelConn, error) {
	var err error
	ep.lock.Lock()
	if ep.closed {
		err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
	}
	ep.lock.Unlock()
	if err != nil {
		return nil, err
	}
	// TODO: make sure IPV6 works
	var d net.Dialer
	netConn, err := d.DialContext(ctx, "tcp", ep.ced.Path)
	if err != nil {
		return nil, fmt.Errorf("%s: DialContext failed: %s", ep.Logger.Prefix(), err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create SocketConn: %s", ep.Logger.Prefix(), err)
	}
	return conn, nil
}
