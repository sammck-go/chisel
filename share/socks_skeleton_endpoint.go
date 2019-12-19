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

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *SocksSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {
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

// DialAndServe initiates a new connection to a Called Service as specified in the
// endpoint configuration, then services the connection using an already established
// callerConn as the proxied Caller's end of the session. This call does not return until
// the bridged session completes or an error occurs. The context may be used to cancel
// connection or servicing of the active session.
// Ownership of callerConn is transferred to this function, and it will be closed before
// this function returns, regardless of whether an error occurs.
// This API may be more efficient than separately using Dial() and then bridging between the two
// ChannelConns with BasicBridgeChannels. In particular, "loop" endpoints can avoid creation
// of a socketpair and an extra bridging goroutine, by directly coupling the acceptor ChannelConn
// to the dialer ChannelConn.
// The return value is a tuple consisting of:
//        Number of bytes sent from callerConn to the dialed calledServiceConn
//        Number of bytes sent from the dialed calledServiceConn callerConn
//        An error, if one occured during dial or copy in either direction
func (ep *SocksSkeletonEndpoint) DialAndServe(
	ctx context.Context,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	calledServiceConn, err := ep.Dial(ctx, extraData)
	if err != nil {
		callerConn.Close()
		return 0, 0, err
	}
	return BasicBridgeChannels(ctx, ep.Logger, callerConn, calledServiceConn)
}
