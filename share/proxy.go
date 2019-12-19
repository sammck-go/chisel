package chshare

import (
	"context"
	"encoding/json"

	"golang.org/x/crypto/ssh"
)

// GetSSHConn is a callback that is used to defer fetching of the ssh.Conn
// until after it is established
type GetSSHConn func() ssh.Conn

// TCPProxy proxies a single channel between a local stub endpoint
// and a remote skeleton endpoint
type TCPProxy struct {
	*Logger
	localChannelEnv LocalChannelEnv
	id              int
	count           int
	chd             *ChannelDescriptor
	ep              LocalStubChannelEndpoint
}

// NewTCPProxy creates a new TCPProxy
func NewTCPProxy(logger *Logger, localChannelEnv LocalChannelEnv, index int, chd *ChannelDescriptor) *TCPProxy {
	id := index + 1
	return &TCPProxy{
		Logger:          logger.Fork("proxy#%d:%s", id, chd),
		localChannelEnv: localChannelEnv,
		id:              id,
		chd:             chd,
	}
}

// Start starts a listener for the local stub endpoint in the backgroud
func (p *TCPProxy) Start(ctx context.Context) error {
	// TODO this should be synchronous and not return until done, or
	// acceptLoop should not be included
	ep, err := NewLocalStubChannelEndpoint(p.Logger, p.localChannelEnv, p.chd.Stub)
	if err != nil {
		return p.Errorf("Unable to create Stub endpoint from descriptor %s: %s", p.chd.Stub, err)
	}
	err = ep.StartListening()
	if err != nil {
		return p.Errorf("StartListening failed for %s: %s", p.chd.Stub, err)
	}
	p.ep = ep

	go p.acceptLoop(ctx)

	return nil
}

func (p *TCPProxy) acceptLoop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.Infof("Forcing close of listening endpoint %s: %s", p.chd.Stub, ctx.Err())
			p.ep.Close()
			p.Debugf("Done forcing close of listening endpoint")
		case <-done:
		}
	}()
	for {
		callerConn, err := p.ep.Accept(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
			default:
				p.Infof("Accept error from %s, shutting down accept loop: %s", p.chd.Stub, err)
			}
			close(done)
			return
		}
		go p.runWithLocalCallerConn(ctx, callerConn)
	}
}

func (p *TCPProxy) runWithLocalCallerConn(ctx context.Context, callerConn ChannelConn) error {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	p.count++

	p.Debugf("TCPProxy Open, getting remote connection")
	sshPrimaryConn, err := p.localChannelEnv.GetSSHConn()
	if err != nil {
		return p.DebugErrorf("Unable to fetch sshPrimaryConn , exiting proxy: %s", err)
	}

	if sshPrimaryConn == nil {
		callerConn.Close()
		return p.DebugErrorf("SSH primary connection, exiting proxy")
	}

	//ssh request for tcp connection for this proxy's remote skeleton endpoint
	skeletonEndpointJSON, err := json.Marshal(p.chd.Skeleton)
	if err != nil {
		callerConn.Close()
		return p.DebugErrorf("Unable to serialize endpoint descriptor '%s': %s", p.chd.Skeleton, err)
	}

	serviceSSHConn, reqs, err := sshPrimaryConn.OpenChannel("chisel", skeletonEndpointJSON)
	if err != nil {
		callerConn.Close()
		return p.DebugErrorf("SSH open channel to remote endpoint %s failed: %s", p.chd.Skeleton, err)
	}

	// will terminate when serviceSSHConn is closed
	go ssh.DiscardRequests(reqs)

	serviceConn, err := NewSSHConn(p.Logger, serviceSSHConn)
	if err != nil {
		sshCloseErr := serviceSSHConn.Close()
		if sshCloseErr != nil {
			p.Debugf("Cose of ssh.Conn failed, ignoring: %s", sshCloseErr)
		}
		callerConn.Close()
		return p.DebugErrorf("SSH open channel to remote endpoint %s failed: %s", p.chd.Skeleton, err)
	}

	callerToService, serviceToCaller, err := BasicBridgeChannels(subCtx, p.Logger, callerConn, serviceConn)
	if err == nil {
		p.Debugf("Proxy Connection for %s ended normally, caller sent %d bytes, service sent %d bytes",
			p.chd, callerToService, serviceToCaller)
	} else {
		return p.DebugErrorf("Proxy conn for %s failed after %d bytes to service, %d bytes to caller: %s",
			p.chd, callerToService, serviceToCaller, err)
	}
	return nil
}
