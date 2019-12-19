package chshare

import (
	"fmt"
	"net"
	"sync/atomic"
)

// SocketConn implements a local TCP or Unix Domain ChannelConn
type SocketConn struct {
	BasicConn
	netConn net.Conn
}

// NewSocketConn creates a new SocketConn
func NewSocketConn(logger *Logger, netConn net.Conn) (*SocketConn, error) {
	id := AllocBasicConnID()
	c := &SocketConn{
		BasicConn: BasicConn{
			Logger: logger.Fork("SocketConn#%d(%s)", id, netConn.RemoteAddr()),
			id:     id,
			Done:   make(chan struct{}),
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
	c.CloseOnce.Do(func() {
		err := c.netConn.Close()
		if err != nil {
			err = fmt.Errorf("%s: %s", c.Logger.Prefix(), err)
		}
		c.CloseErr = err
		close(c.Done)
	})

	<-c.Done
	return c.CloseErr
}

// WaitForClose blocks until the Close() method has been called and completed
func (c *SocketConn) WaitForClose() error {
	<-c.Done
	return c.CloseErr
}

// Read implements the Reader interface
func (c *SocketConn) Read(p []byte) (n int, err error) {
	n, err = c.netConn.Read(p)
	atomic.AddInt64(&c.NumBytesRead, int64(n))
	return n, err
}

// Write implements the Writer interface
func (c *SocketConn) Write(p []byte) (n int, err error) {
	n, err = c.netConn.Write(p)
	atomic.AddInt64(&c.NumBytesWritten, int64(n))
	return n, err
}
