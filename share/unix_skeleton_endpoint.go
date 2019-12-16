package chshare

import (
	"context"
	"fmt"
	"net"
)

// UnixSkeletonEndpoint implements a local Unix skeleton
type UnixSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
}

// NewUnixSkeletonEndpoint creates a new UnixSkeletonEndpoint
func NewUnixSkeletonEndpoint(logger *Logger, ced *ChannelEndpointDescriptor) (*UnixSkeletonEndpoint, error) {
	ep := &UnixSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("UnixSkeletonEndpoint: %s", ced),
			ced:    ced,
		},
	}
	return ep, nil
}

func (ep *UnixSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *UnixSkeletonEndpoint) Close() error {
	ep.lock.Lock()
	if !ep.closed {
		ep.closed = true
	}
	ep.lock.Unlock()
	return nil
}

// DialContext initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *UnixSkeletonEndpoint) DialContext(ctx context.Context, extraData []byte) (ChannelConn, error) {
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
	netConn, err := d.DialContext(ctx, "Unix", ep.ced.Path)
	if err != nil {
		return nil, fmt.Errorf("%s: DialContext failed: %s", ep.Logger.Prefix(), err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create SocketConn: %s", ep.Logger.Prefix(), err)
	}
	return conn, nil
}
