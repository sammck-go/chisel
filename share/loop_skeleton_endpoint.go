package chshare

import (
	"context"
	"fmt"
)

// LoopSkeletonEndpoint implements a local Loop skeleton
type LoopSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	loopServer *LoopServer
}

// NewLoopSkeletonEndpoint creates a new LoopSkeletonEndpoint
func NewLoopSkeletonEndpoint(
	logger *Logger,
	ced *ChannelEndpointDescriptor,
	loopServer *LoopServer,
) (*LoopSkeletonEndpoint, error) {
	ep := &LoopSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("LoopSkeletonEndpoint: %s", ced),
			ced:    ced,
		},
		loopServer: loopServer,
	}
	return ep, nil
}

func (ep *LoopSkeletonEndpoint) String() string {
	return ep.Logger.Prefix()
}

// GetLoopPath returns the loop pathname associated with this LoopStubEndpoint
func (ep *LoopSkeletonEndpoint) GetLoopPath() string {
	return ep.ced.Path
}

// Close implements the Closer interface
func (ep *LoopSkeletonEndpoint) Close() error {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.closed = true
	return nil
}

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *LoopSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {
	var err error
	ep.lock.Lock()
	if ep.closed {
		err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
	}
	ep.lock.Unlock()
	if err != nil {
		return nil, err
	}
	return ep.loopServer.Dial(ctx, ep.GetLoopPath(), extraData)
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
func (ep *LoopSkeletonEndpoint) DialAndServe(
	ctx context.Context,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	var err error
	ep.lock.Lock()
	if ep.closed {
		err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
	}
	ep.lock.Unlock()
	if err != nil {
		return 0, 0, err
	}
	return ep.loopServer.DialAndServe(ctx, ep.GetLoopPath(), callerConn, extraData)
}
