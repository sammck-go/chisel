package chshare

import (
	"fmt"
	"net"
)

// SocketConn implements a local TCP or Unix Domain ChannelConn
type SocketConn struct {
	BasicConn
	netConn net.Conn
}

// NewSocketConn creates a new SocketConn
func NewSocketConn(logger *Logger, netConn net.Conn) (*SocketConn, error) {
	c := &SocketConn{
		BasicConn: BasicConn{
			Logger: logger.Fork("SocketConn: %s", netConn),
		},
		netConn: netConn,
	}
	return c, nil
}

func (c *SocketConn) String() string {
	return c.Logger.Prefix()
}

// CloseWrite shuts down the writing side of the "socket". Corresponds to net.TCPConn.CloseWrite().
// this method is called when end-of-stream is reached reading from the other ChannelConn of a pair
// pair are connected via a ChannelPipe. It allows for protocols like HTTP 1.0 in which a client
// sends a request, closes the write side of the socket, then reads the response, and a server reads
// a request until end-of-stream before sending a response. Part of the ChannelConn interface
func (c *SocketConn) CloseWrite() error {
	var err error
	switch nc := c.netConn.(type) {
	case *net.UnixConn:
		err = nc.CloseWrite()
	case *net.TCPConn:
		err = nc.CloseWrite()
	default:
		err = fmt.Errorf("CloseWrite() called on unknown net.Conn type %T", nc)
	}
	if err != nil {
		err = fmt.Errorf("%s: %s", c.Logger.Prefix(), err)
	}
	return err
}

// Close implements the Closer interface
func (c *SocketConn) Close() error {
	return c.netConn.Close()
}

// Read implements the Reader interface
func (c *SocketConn) Read(p []byte) (n int, err error) {
	return c.netConn.Read(p)
}

// Write implements the Writer interface
func (c *SocketConn) Write(p []byte) (n int, err error) {
	return c.netConn.Write(p)
}
