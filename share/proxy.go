package chshare

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

type GetSSHConn func() ssh.Conn

type TCPProxy struct {
	*Logger
	ssh    GetSSHConn
	id     int
	count  int
	chd *ChannelDescriptor
}

func NewTCPProxy(logger *Logger, ssh GetSSHConn, index int, chd *ChannelDescriptor) *TCPProxy {
	id := index + 1
	return &TCPProxy{
		Logger: logger.Fork("proxy#%d:%s", id, chd),
		ssh:    ssh,
		id:     id,
		chd:    chd,
	}
}

func (p *TCPProxy) Start(ctx context.Context) error {
	if p.chd.Stub.Type == ChannelEndpointTypeStdio {
		src := NewStdioPipePair(p.Logger)
		go p.accept(src)
	} else {
		// TODO: support IPV6
		l, err := net.Listen("tcp4", p.chd.Stub.Path)
		if err != nil {
			return fmt.Errorf("%s: TCP listen failed for path '%s': %s", p.Logger.Prefix(), p.chd.Stub.Path, err)
		}
		go p.listen(ctx, l)
	}
	return nil
}

func (p *TCPProxy) listen(ctx context.Context, l net.Listener) {
	p.Infof("Listening")
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
			p.Infof("Closed")
		case <-done:
		}
	}()
	for {
		src, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
			default:
				p.Infof("Accept error: %s", err)
			}
			close(done)
			return
		}
		go p.accept(src)
	}
}

func (p *TCPProxy) accept(src io.ReadWriteCloser) {
	defer src.Close()
	p.count++
	cid := p.count
	l := p.Fork("conn#%d", cid)
	l.Debugf("TCPProxy Open, getting remote connection")
	sshConn := p.ssh()
	if sshConn == nil {
		l.Debugf("No remote connection, exiting proxy")
		return
	}
	//ssh request for tcp connection for this proxy's remote skeleton endpoint
	skeletonEndpointStr := p.chd.Skeleton.String()
	dst, reqs, err := sshConn.OpenChannel("chisel", []byte(skeletonEndpointStr))
	if err != nil {
		l.Infof("Stream error: %s", err)
		return
	}
	go ssh.DiscardRequests(reqs)
	//then pipe
	s, r := Pipe(src, dst)
	l.Debugf("Close (sent %s received %s)", sizestr.ToString(s), sizestr.ToString(r))
}
