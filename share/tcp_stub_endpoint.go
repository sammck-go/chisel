package chshare

import (
	"fmt"
	"net"
)

// TCPStubEndpoint implements a local TCP stub
type TCPStubEndpoint struct {
	// Implements LocalStubChannelEndpoint
	BasicEndpoint
	listenErr error
	listener  net.Listener
}

// NewTCPStubEndpoint creates a new TCPStubEndpoint
func NewTCPStubEndpoint(logger *Logger, ced *ChannelEndpointDescriptor) (*TCPStubEndpoint, error) {
	ep := &TCPStubEndpoint{
		BasicEndpoint: BasicEndpoint{
			Logger: logger.Fork("TCPStubEndpoint: %s", ced),
			ced:    ced,
		},
	}
	return ep, nil
}

func (ep *TCPStubEndpoint) String() string {
	return ep.Logger.Prefix()
}

// Close implements the Closer interface
func (ep *TCPStubEndpoint) Close() error {
	// TODO: better synchronization
	var listener net.Listener
	ep.lock.Lock()
	if !ep.closed {
		listener = ep.listener
		ep.listener = nil
		ep.closed = true
	}
	ep.lock.Unlock()

	var err error
	if listener != nil {
		err = listener.Close()
	}
	return err
}

func (ep *TCPStubEndpoint) getListener() (net.Listener, error) {
	var listener net.Listener
	var err error

	ep.lock.Lock()
	{
		if ep.closed {
			err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
		} else if ep.listener == nil && ep.listenErr == nil {
			// TODO: support IPV6
			listener, err = net.Listen("tcp4", ep.ced.Path)
			if err != nil {
				err = fmt.Errorf("%s: TCP listen failed for path '%s': %s", ep.Logger.Prefix(), ep.ced.Path, err)
			} else {
				ep.listener = listener
			}
			ep.listenErr = err
		} else {
			listener = ep.listener
			err = ep.listenErr
		}
	}
	ep.lock.Unlock()

	return listener, err
}

// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to begin accepting Callers before you make the first Accept call. Part of
// AcceptorChannelEndpoint interface.
func (ep *TCPStubEndpoint) StartListening() error {
	_, err := ep.getListener()
	return err
}

// Accept listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration. This call does not return until a new connection is available or a
// error occurs. There is no way to cancel an Accept() request other than closing the endpoint. Part of
// the AcceptorChannelEndpoint interface.
func (ep *TCPStubEndpoint) Accept() (ChannelConn, error) {
	listener, err := ep.getListener()
	if err != nil {
		return nil, err
	}

	netConn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("%s: Accept failed: %s", ep.Logger.Prefix(), err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create SocketConn: %s", ep.Logger.Prefix(), err)
	}
	return conn, nil
}

