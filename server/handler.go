package chserver

import (
	"context"
	"encoding/json"
	chshare "github.com/XevoInc/chisel/share"
	socks5 "github.com/armon/go-socks5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// handleClientHandler is the main http websocket handler for the chisel server
func (s *Server) handleClientHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	//websockets upgrade AND has chisel prefix
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	if upgrade == "websocket" {
		protocol := r.Header.Get("Sec-WebSocket-Protocol")
		if strings.HasPrefix(protocol, "xevo-chisel-") {
			if protocol == chshare.ProtocolVersion {
				s.Debugf("Upgrading to websocket, URL tail=\"%s\", protocol=\"%s\"", r.URL.String(), protocol)
				wsConn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					err = s.DebugErrorf("Failed to upgrade to websocket: %s", err)
					http.Error(w, err.Error(), 503)
					return
				}

				go func() {
					s.handleWebsocket(ctx, wsConn)
					wsConn.Close()
				}()

				return
			}

			s.Infof("Client connection using unsupported websocket protocol '%s', expected '%s'",
				protocol, chshare.ProtocolVersion)

			http.Error(w, "Not Found", 404)
			return
		}
	}

	//proxy target was provided
	if s.reverseProxy != nil {
		s.reverseProxy.ServeHTTP(w, r)
		return
	}

	//no proxy defined, provide access to health/version checks
	switch r.URL.String() {
	case "/health":
		w.Write([]byte("OK\n"))
		return
	case "/version":
		w.Write([]byte(chshare.BuildVersion))
		return
	}

	http.Error(w, "Not Found", 404)
}

// ServerSSHSession wraps a primary SSH connection with a single client proxy
type ServerSSHSession struct {
	*chshare.Logger

	// Server is the chisel proxy server on which this session is running
	server *Server

	// id is a unique id of this session, for logging purposes
	id int32

	// sshConn is the server-side ssh session connection
	sshConn *ssh.ServerConn

	// chans is the chan on which connect requests from remote stub to local endpoint are eceived
	chans <-chan ssh.NewChannel

	// reqs is the chan on which ssh requests are received (including initial config request)
	reqs <-chan *ssh.Request

	// done is closed at completion of Run
	done chan struct{}
}

// NewServerSSHSession creates a server-side proxy session object
func NewServerSSHSession(server *Server) (*ServerSSHSession, error) {
	id := server.AllocSessionID()
	s := &ServerSSHSession{
		Logger: server.Logger.Fork("SSHSession#%d", id),
		server: server,
		id:     id,
		done:   make(chan struct{}),
	}
	return s, nil
}

// Implement LocalChannelEnv

// IsServer returns true if this is a proxy server; false if it is a cliet
func (s *ServerSSHSession) IsServer() bool {
	return true
}

// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
func (s *ServerSSHSession) GetLoopServer() *chshare.LoopServer {
	return s.server.loopServer
}

// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
// nil otherwise
func (s *ServerSSHSession) GetSocksServer() *socks5.Server {
	return s.server.socksServer
}

// GetSSHConn waits for and returns the main ssh.Conn that this proxy is using to
// communicate with the remote proxy. It is possible that goroutines servicing
// local stub sockets will ask for this before it is available (if for example
// a listener on the client accepts a connection before the server has ackknowledged
// configuration. An error response indicates that the SSH connection failed to initialize.
func (s *ServerSSHSession) GetSSHConn() (ssh.Conn, error) {
	return s.sshConn, nil
}

// receiveSSHRequest receives a single SSH request from the ssh.ServerConn. Can be
// canceled with the context
func (s *ServerSSHSession) receiveSSHRequest(ctx context.Context) (*ssh.Request, error) {
	select {
	case r := <-s.reqs:
		return r, nil
	case <-ctx.Done():
		return nil, s.DebugErrorf("SSH request not received: %s", ctx.Err())
	}
}

// sendSSHReply sends a reply to an SSH request received from ssh.ServerConn.
// If the context is cancelled before the response is sent, a goroutine will leak
// until the ssh.ServerConn is closed (which should come quickly due to err returned)
func (s *ServerSSHSession) sendSSHReply(ctx context.Context, r *ssh.Request, ok bool, payload []byte) error {
	// TODO: currently no way to cancel the send without closing the sshConn
	result := make(chan error)

	go func() {
		err := r.Reply(ok, payload)
		result <- err
		close(result)
	}()

	var err error

	select {
	case err = <-result:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		err = s.DebugErrorf("SSH repy send failed: %s", err)
	}

	return err
}

// sendSSHErrorReply sends an error reply to an SSH request received from ssh.ServerConn.
// If the context is cancelled before the response is sent, a goroutine will leak
// until the ssh.ServerConn is closed (which should come quickly due to err returned)
func (s *ServerSSHSession) sendSSHErrorReply(ctx context.Context, r *ssh.Request, err error) error {
	s.Debugf("Sending SSH error reply: %s", err)
	return s.sendSSHReply(ctx, r, false, []byte(err.Error()))
}

// runWithSSHConn runs a proxy session from a client from start to end, given
// an incoming ssh.ServerConn. On exit, the incoming ssh.ServerConn still
// needs to be closed.
func (s *ServerSSHSession) runWithSSHConn(
	ctx context.Context,
	sshConn *ssh.ServerConn,
	chans <-chan ssh.NewChannel,
	reqs <-chan *ssh.Request,
) error {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	s.sshConn = sshConn
	s.chans = s.chans
	s.reqs = reqs

	// pull the users from the session map
	var user *chshare.User
	if s.server.users.Len() > 0 {
		sid := string(sshConn.SessionID())
		user, _ = s.server.sessions.Get(sid)
		s.server.sessions.Del(sid)
	}

	//verify configuration
	s.Debugf("Receiving configuration")
	// wait for configuration request, with timeout
	cfgCtx, cfgCtxCancel := context.WithTimeout(subCtx, 10*time.Second)
	r, err := s.receiveSSHRequest(cfgCtx)
	cfgCtxCancel()
	if err != nil {
		return s.DebugErrorf("receiveSSHRequest failed: %s", err)
	}

	s.Debugf("Received SSH Req")

	// convenience function to send an error reply and return
	// the original error. Ignores failures sending the reply
	// since we will be bailing out anyway
	failed := func(err error) error {
		s.sendSSHErrorReply(subCtx, r, err)
		return err
	}

	if r.Type != "config" {
		return failed(s.DebugErrorf("Expecting \"config\" request, got \"%s\"", r.Type))
	}

	c := &chshare.SessionConfigRequest{}
	err = c.Unmarshal(r.Payload)
	if err != nil {
		return failed(s.DebugErrorf("Invalid session config request encoding: %s", err))
	}

	//print if client and server  versions dont match
	if c.Version != chshare.BuildVersion {
		v := c.Version
		if v == "" {
			v = "<unknown>"
		}
		s.Infof("WARNING: Chisel Client version (%s) differs from server version (%s)", v, chshare.BuildVersion)
	}

	//confirm reverse tunnels are allowed
	for _, chd := range c.ChannelDescriptors {
		if chd.Reverse && !s.server.reverseOk {
			return failed(s.DebugErrorf("Reverse port forwarding not enabled on server"))
		}
	}
	//if user is provided, ensure they have
	//access to the desired remotes
	if user != nil {
		for _, chd := range c.ChannelDescriptors {
			chdString := chd.String()
			if !user.HasAccess(chdString) {
				return failed(s.DebugErrorf("Access to \"%s\" denied", chdString))
			}
		}
	}

	//set up reverse port forwarding
	for i, chd := range c.ChannelDescriptors {
		if chd.Reverse {
			s.Debugf("Reverse-mode route[%d] %s; starting stub listener", i, chd.String())
			proxy := chshare.NewTCPProxy(s.Logger, func() ssh.Conn { return sshConn }, i, chd)
			if err := proxy.Start(subCtx); err != nil {
				return failed(s.DebugErrorf("Unable to start stub listener %s: %s", chd.String(), err))
			}
		} else {
			s.Debugf("Forward-mode route[%d] %s; connections will be created on demand", i, chd.String())
		}
	}

	//success!
	err = s.sendSSHReply(subCtx, r, true, nil)
	if err != nil {
		return s.DebugErrorf("Failed to send SSH config success response: %s", err)
	}

	go s.handleSSHRequests(subCtx, reqs)
	go s.handleSSHChannels(subCtx, chans)

	s.Debugf("SSH session up and running")

	return sshConn.Wait()
}

// Run runs an SSH session to completion from an incoming
// just-connected client socket (which has already been wrapped on a websocket)
// The incoming conn is not
func (s *ServerSSHSession) Run(ctx context.Context, conn net.Conn) error {
	s.Debugf("SSH Handshaking...")
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.server.sshConfig)
	if err != nil {
		s.Debugf("Failed to handshake (%s)", err)
		close(s.done)
		return err
	}

	err = s.runWithSSHConn(ctx, sshConn, chans, reqs)
	if err != nil {
		s.Debugf("SSH session failed: %s", err)
	}

	s.Debugf("Closing SSH connection")
	sshConn.Close()
	close(s.done)
	return err
}

// AllocSessionID allocates a monotonically incresing session ID number (for debugging/logging only)
func (s *Server) AllocSessionID() int32 {
	id := atomic.AddInt32(&s.sessCount, 1)
	return id
}

// handleWebsocket handles an incoming client request that is intended tois responsible for handling the websocket connection
// It upgrades . It is guaranteed on return
//
func (s *Server) handleWebsocket(ctx context.Context, wsConn *websocket.Conn) {
	session, err := NewServerSSHSession(s)
	if err != nil {
		session.Debugf("Failed to create ServerSSHSession: %s", err)
		return
	}
	conn := chshare.NewWebSocketConn(wsConn)
	session.Run(ctx, conn)
	conn.Close() // closes the websocket too
}

func (s *ServerSSHSession) handleSSHRequests(ctx context.Context, reqs <-chan *ssh.Request) {
	for {
		select {
		case req := <-reqs:
			if req == nil {
				s.Debugf("End of incoming SSH request stream")
				return
			} else {
				switch req.Type {
				case "ping":
					err := s.sendSSHReply(ctx, req, true, nil)
					if err != nil {
						s.Debugf("SSH ping reply send failed, ignoring: %s", err)
					}
				default:
					err := s.DebugErrorf("Unknown SSH request type: %s", req.Type)
					err = s.sendSSHErrorReply(ctx, req, err)
					if err != nil {
						s.Debugf("SSH send reply for unknown request type failed, ignoring: %s", err)
					}
				}
			}
		case <-ctx.Done():
			s.Debugf("SSH request stream processing aborted: %s", ctx.Err())
			return
		}
	}
}

// handleSSHNewChannel handles an incoming ssh.NewCHannel request from beginning to end
// It is intended to run in its own goroutine, so as to not block other
// SSH activity
func (s *ServerSSHSession) handleSSHNewChannel(ctx context.Context, ch ssh.NewChannel) error {
	reject := func(reason ssh.RejectionReason, err error) error {
		s.Debugf("Sending SSH NewChannel rejection (reason=%v): %s", reason, err)
		// TODO allow cancellation with ctx
		rejectErr := ch.Reject(reason, err.Error())
		if rejectErr != nil {
			s.Debugf("Unable to send SSH NewChannel reject response, ignoring: %s", rejectErr)
		}
		return err
	}
	epdJSON := ch.ExtraData()
	epd := &chshare.ChannelEndpointDescriptor{}
	err := json.Unmarshal(epdJSON, epd)
	if err != nil {
		return reject(ssh.UnknownChannelType, s.server.Errorf("Badly formatted NewChannel request"))
	}
	s.Debugf("SSH NewChannel request, endpoint ='%s'", epd.String())
	ep, err := chshare.NewLocalSkeletonChannelEndpoint(s.Logger, s, epd)
	if err != nil {
		s.Debugf("Failed to create skeleton endpoint for SSH NewChannel: %s", err)
		return reject(ssh.Prohibited, err)
	}

	// TODO: The actual local connect request should succeed before we accept the remote request.
	//       Need to refactor code here
	// TODO: Allow cancellation with ctx
	sshChannel, reqs, err := ch.Accept()
	if err != nil {
		s.Debugf("Failed to accept SSH NewChannel: %s", err)
		ep.Close()
		return err
	}

	// This will shut down when sshChannel is closed
	go ssh.DiscardRequests(reqs)

	// wrap the ssh.Channel to look like a ChannelConn
	sshConn, err := chshare.NewSSHConn(s.Logger, sshChannel)
	if err != nil {
		s.Debugf("Failed wrap SSH NewChannel: %s", err)
		sshChannel.Close()
		ep.Close()
		return err
	}

	// sshChannel is now wrapped by sshConn, and will be closed when sshConn is closed

	var extraData []byte
	numSent, numReceived, err := ep.DialAndServe(ctx, sshConn, extraData)

	// sshConn and sshChannel have now been closed

	if err != nil {
		s.Debugf("NewChannel session ended with error after %d bytes (caller->called), %d bytes (called->caller): %s", numSent, numReceived, err)
	} else {
		s.Debugf("NewChannel session ended normally after %d bytes (caller->called), %d bytes (called->caller)", numSent, numReceived)
	}

	return err
}

func (s *ServerSSHSession) handleSSHChannels(ctx context.Context, newChannels <-chan ssh.NewChannel) {
	for {
		select {
		case ch := <-newChannels:
			if ch == nil {
				s.Debugf("End of incoming SSH NewChannels stream")
				return
			} else {
				go s.handleSSHNewChannel(ctx, ch)
			}
		case <-ctx.Done():
			s.Debugf("SSH NewChannels stream processing aborted: %s", ctx.Err())
			return
		}
	}
}

func (s *Server) handleSocksStream(l *chshare.Logger, src io.ReadWriteCloser) {
	conn := chshare.NewRWCConn(src)
	s.connStats.Open()
	l.Debugf("%s Opening", s.connStats)
	err := s.socksServer.ServeConn(conn)
	s.connStats.Close()
	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		l.Debugf("%s: Closed (error: %s)", s.connStats, err)
	} else {
		l.Debugf("%s: Closed", s.connStats)
	}
}
