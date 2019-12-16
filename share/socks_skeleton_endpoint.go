package chshare

import (
	"context"
	"fmt"

	socks5 "github.com/armon/go-socks5"
	"github.com/prep/socketpair"
)

// SocksSkeletonEndpoint implements a local Socks skeleton
type SocksSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	socksServer *socks5.Server
}

// NewSocksSkeletonEndpoint creates a new SocksSkeletonEndpoint
func NewSocksSkeletonEndpoint(
	logger *Logger,
	ced *ChannelEndpointDescriptor,
	socksServer *socks5.Server,
) (*SocksSkeletonEndpoint, error) {
	ep := &SocksSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("SocksSkeletonEndpoint: %s", ced),
			ced:    ced,
		},
	}
	return ep, nil
}

func (ep *SocksSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *SocksSkeletonEndpoint) Close() error {
	ep.lock.Lock()
	if !ep.closed {
		ep.closed = true
	}
	ep.lock.Unlock()
	return nil
}

// DialContext initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *SocksSkeletonEndpoint) DialContext(ctx context.Context, extraData []byte) (ChannelConn, error) {
	var err error
	ep.lock.Lock()
	if ep.closed {
		err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
	}
	ep.lock.Unlock()
	if err != nil {
		return nil, err
	}

	// Create a socket pair so that the socks5 server has something to talk to and
	// we have something to return to the caller. This results in one hop through a socket
	// but it preserves our abstraction that requires endpoints to create their ChannelConn
	// first, then we wire them together with a pipe task.
	netConn, socksNetConn, err := socketpair.New("unix")
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create socketpair: %s", ep.Logger.Prefix(), err)
	}

	// Now we can create a ChannelCon for our end of the connection
	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		netConn.Close()
		socksNetConn.Close()
		return nil, fmt.Errorf("%s: Unable to wrap net.Conn with SocketConn: %s", ep.Logger.Prefix(), err)
	}

	err = ep.socksServer.ServeConn(socksNetConn)
	if err != nil {
		socksNetConn.Close()
		conn.Close()
		return nil, fmt.Errorf("%s: Socks5 server refused connect: %s", ep.Logger.Prefix(), err)
	}

	return conn, nil
}
