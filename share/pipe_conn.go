package chshare

import (
	"io"
)

// PipeConn implements a local ChannelConn from a read stream and a write stream (e.g., stdin and stdout)
type PipeConn struct {
	BasicConn
	input       io.ReadCloser
	output      io.WriteCloser
	writeClosed bool
}

// NewPipeConn creates a new PipeConn
func NewPipeConn(logger *Logger, input io.ReadCloser, output io.WriteCloser) (*PipeConn, error) {
	c := &PipeConn{
		BasicConn: BasicConn{
			Logger: logger.Fork("PipeConn: (%s->%s)", input, output),
		},
		input:  input,
		output: output,
	}
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
	err := c.output.Close()
	return err
}

// Close implements the Closer interface
func (c *PipeConn) Close() error {
	c.output.Close()
	// ignore errors on output close (may have been shutdown with CloseWrite())
	err := c.input.Close()
	return err
}

// Read implements the Reader interface
func (c *PipeConn) Read(p []byte) (n int, err error) {
	return c.input.Read(p)
}

// Write implements the Writer interface
func (c *PipeConn) Write(p []byte) (n int, err error) {
	return c.output.Write(p)
}
