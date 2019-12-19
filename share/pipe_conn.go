package chshare

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// PipeConn implements a local ChannelConn from a read stream and a write stream (e.g., stdin and stdout)
type PipeConn struct {
	BasicConn
	input          io.ReadCloser
	output         io.WriteCloser
	closeWriteOnce sync.Once
	closeWriteErr  error
}

// NewPipeConn creates a new PipeConn
func NewPipeConn(logger *Logger, input io.ReadCloser, output io.WriteCloser) (*PipeConn, error) {
	c := &PipeConn{
		input:  input,
		output: output,
	}
	c.InitBasicConn(logger, "PipeConn(%s->%s)", input, output)
	return c, nil
}

func (c *PipeConn) String() string {
	return c.Logger.Prefix()
}

// CloseWrite shuts down the writing side of the "Pipe". Corresponds to net.TCPConn.CloseWrite().
// this method is called when end-of-stream is reached reading from the other ChannelConn of a pair
// pair are connected via a ChannelPipe. It allows for protocols like HTTP 1.0 in which a client
// sends a request, closes the write side of the Pipe, then reads the response, and a server reads
// a request until end-of-stream before sending a response. Part of the ChannelConn interface
func (c *PipeConn) CloseWrite() error {
	c.closeWriteOnce.Do(func() {
		c.closeWriteErr = c.output.Close()
	})
	return c.closeWriteErr
}

// Close implements the Closer interface
func (c *PipeConn) Close() error {
	c.CloseOnce.Do(func() {

		// ignore errors on output close (may have been shutdown with CloseWrite())
		err := c.input.Close()
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
func (c *PipeConn) WaitForClose() error {
	<-c.Done
	return c.CloseErr
}

// Read implements the Reader interface
func (c *PipeConn) Read(p []byte) (n int, err error) {
	n, err = c.input.Read(p)
	atomic.AddInt64(&c.NumBytesRead, int64(n))
	return n, err
}

// Write implements the Writer interface
func (c *PipeConn) Write(p []byte) (n int, err error) {
	n, err = c.output.Write(p)
	atomic.AddInt64(&c.NumBytesWritten, int64(n))
	return n, err
}
