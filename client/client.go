package chclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	chshare "github.com/XevoInc/chisel/share"
	socks5 "github.com/armon/go-socks5"
	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
	"golang.org/x/crypto/ssh"
)

//Config represents a client configuration
type Config struct {
	shared           *chshare.Config
	Fingerprint      string
	Auth             string
	KeepAlive        time.Duration
	MaxRetryCount    int
	MaxRetryInterval time.Duration
	Server           string
	HTTPProxy        string
	ChdStrings       []string
	HostHeader       string
}

//Client represents a client instance
type Client struct {
	*chshare.Logger
	config       *Config
	sshConfig    *ssh.ClientConfig
	sshConn      ssh.Conn
	httpProxyURL *url.URL
	server       string
	running      bool
	runningc     chan error
	connStats    chshare.ConnStats
	socksServer  *socks5.Server
	loopServer   *chshare.LoopServer
}

//NewClient creates a new client instance
func NewClient(config *Config) (*Client, error) {
	//apply default scheme
	logger := chshare.NewLogger("client")

	if !strings.HasPrefix(config.Server, "http") {
		config.Server = "http://" + config.Server
	}
	if config.MaxRetryInterval < time.Second {
		config.MaxRetryInterval = 5 * time.Minute
	}
	u, err := url.Parse(config.Server)
	if err != nil {
		return nil, err
	}
	//apply default port
	if !regexp.MustCompile(`:\d+$`).MatchString(u.Host) {
		if u.Scheme == "https" || u.Scheme == "wss" {
			u.Host = u.Host + ":443"
		} else {
			u.Host = u.Host + ":80"
		}
	}
	//swap to websockets scheme
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	shared := &chshare.Config{}
	for _, s := range config.ChdStrings {
		chd, err := chshare.ParseChannelDescriptor(s)
		if err != nil {
			return nil, fmt.Errorf("%s: Failed to parse channel descriptor string '%s': %s", logger.Prefix(), s, err)
		}
		shared.ChannelDescriptors = append(shared.ChannelDescriptors, chd)
	}
	config.shared = shared
	loopServer, err := chshare.NewLoopServer(logger)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to start loop server", logger.Prefix())
	}
	client := &Client{
		Logger:     logger,
		config:     config,
		server:     u.String(),
		running:    true,
		runningc:   make(chan error, 1),
		loopServer: loopServer,
	}
	client.Info = true

	if p := config.HTTPProxy; p != "" {
		client.httpProxyURL, err = url.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("%s: Invalid proxy URL (%s)", logger.Prefix(), err)
		}
	}

	user, pass := chshare.ParseAuth(config.Auth)

	client.sshConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		ClientVersion:   "SSH-" + chshare.ProtocolVersion + "-client",
		HostKeyCallback: client.verifyServer,
		Timeout:         30 * time.Second,
	}

	return client, nil
}

// Implement LocalChannelEnv interface

// IsServer returns true if this is a proxy server; false if it is a cliet
func (c *Client) IsServer() bool {
	return false
}

// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
func (c *Client) GetLoopServer() *chshare.LoopServer {
	return c.loopServer
}

// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
// nil otherwise
func (c *Client) GetSocksServer() *socks5.Server {
	return c.socksServer
}

//Run starts client and blocks while connected
func (c *Client) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		return err
	}
	return c.Wait()
}

func (c *Client) verifyServer(hostname string, remote net.Addr, key ssh.PublicKey) error {
	expect := c.config.Fingerprint
	got := chshare.FingerprintKey(key)
	if expect != "" && !strings.HasPrefix(got, expect) {
		return fmt.Errorf("Invalid fingerprint (%s)", got)
	}
	//overwrite with complete fingerprint
	c.Infof("Fingerprint %s", got)
	return nil
}

//Start client and does not block
func (c *Client) Start(ctx context.Context) error {
	via := ""
	if c.httpProxyURL != nil {
		via = " via " + c.httpProxyURL.String()
	}
	//prepare non-reverse proxies (other than stdio proxy, which we defer til we have a good connection)
	for i, chd := range c.config.shared.ChannelDescriptors {
		if !chd.Reverse && chd.Stub.Type != chshare.ChannelEndpointTypeStdio {
			proxy := chshare.NewTCPProxy(c.Logger, func() ssh.Conn { return c.sshConn }, i, chd)
			if err := proxy.Start(ctx); err != nil {
				return err
			}
		}
	}
	c.Infof("Connecting to %s%s\n", c.server, via)
	//optional keepalive loop
	if c.config.KeepAlive > 0 {
		go c.keepAliveLoop()
	}
	//connection loop
	go c.connectionLoop(ctx)
	return nil
}

func (c *Client) keepAliveLoop() {
	for c.running {
		time.Sleep(c.config.KeepAlive)
		if c.sshConn != nil {
			c.sshConn.SendRequest("ping", true, nil)
		}
	}
}

func (c *Client) connectionLoop(ctx context.Context) {
	//connection loop!
	var connerr error
	stdioStarted := false
	b := &backoff.Backoff{Max: c.config.MaxRetryInterval}
	for c.running {
		if connerr != nil {
			attempt := int(b.Attempt())
			maxAttempt := c.config.MaxRetryCount
			d := b.Duration()
			//show error and attempt counts
			msg := fmt.Sprintf("Connection error: %s", connerr)
			if attempt > 0 {
				msg += fmt.Sprintf(" (Attempt: %d", attempt)
				if maxAttempt > 0 {
					msg += fmt.Sprintf("/%d", maxAttempt)
				}
				msg += ")"
			}
			c.Debugf(msg)
			//give up?
			if maxAttempt >= 0 && attempt >= maxAttempt {
				break
			}
			c.Infof("Retrying in %s...", d)
			connerr = nil
			chshare.SleepSignal(d)
		}
		d := websocket.Dialer{
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
			HandshakeTimeout: 45 * time.Second,
			Subprotocols:     []string{chshare.ProtocolVersion},
		}
		//optionally CONNECT proxy
		if c.httpProxyURL != nil {
			d.Proxy = func(*http.Request) (*url.URL, error) {
				return c.httpProxyURL, nil
			}
		}
		wsHeaders := http.Header{}
		if c.config.HostHeader != "" {
			wsHeaders = http.Header{
				"Host": {c.config.HostHeader},
			}
		}
		wsConn, _, err := d.Dial(c.server, wsHeaders)
		if err != nil {
			connerr = err
			continue
		}
		conn := chshare.NewWebSocketConn(wsConn)
		// perform SSH handshake on net.Conn
		c.Debugf("Handshaking...")
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, "", c.sshConfig)
		if err != nil {
			if strings.Contains(err.Error(), "unable to authenticate") {
				c.Infof("Authentication failed")
				c.Debugf(err.Error())
			} else {
				c.Infof(err.Error())
			}
			break
		}
		c.config.shared.Version = chshare.BuildVersion
		conf, _ := chshare.EncodeConfig(c.config.shared)
		c.Debugf("Sending config")
		t0 := time.Now()
		_, configerr, err := sshConn.SendRequest("config", true, conf)
		if err != nil {
			c.Infof("Config verification failed")
			break
		}
		if len(configerr) > 0 {
			c.Infof(string(configerr))
			break
		}
		c.Infof("Connected (Latency %s)", time.Since(t0))
		//connected
		b.Reset()
		c.sshConn = sshConn
		go ssh.DiscardRequests(reqs)

		if !stdioStarted {
			stdioStarted = true
			//prepare stdio proxy, which we deferred til we had a good connection)
			for i, chd := range c.config.shared.ChannelDescriptors {
				if !chd.Reverse && chd.Stub.Type == chshare.ChannelEndpointTypeStdio {
					proxy := chshare.NewTCPProxy(c.Logger, func() ssh.Conn { return c.sshConn }, i, chd)
					if err := proxy.Start(ctx); err != nil {
						c.Infof("Start of stdio proxy failed: %s", err)
						// TODO: stop the client
					}
					break
				}
			}
		}

		go c.connectStreams(chans)
		err = sshConn.Wait()
		//disconnected
		c.sshConn = nil
		if err != nil && err != io.EOF {
			connerr = err
			continue
		}
		c.Infof("Disconnected\n")
	}
	close(c.runningc)
}

//Wait blocks while the client is running.
//Can only be called once.
func (c *Client) Wait() error {
	return <-c.runningc
}

//Close manually stops the client
func (c *Client) Close() error {
	c.running = false
	if c.sshConn == nil {
		return nil
	}
	return c.sshConn.Close()
}

/*
func (c *Client) connectStreams(chans <-chan ssh.NewChannel) {
	for ch := range chans {
		remote := string(ch.ExtraData())
		stream, reqs, err := ch.Accept()
		if err != nil {
			c.Debugf("Failed to accept stream: %s", err)
			continue
		}
		go ssh.DiscardRequests(reqs)
		l := c.Logger.Fork("conn#%d", c.connStats.New())
		go chshare.HandleTCPStream(l, &c.connStats, stream, remote)
	}
}
*/

func (c *Client) connectStreams(chans <-chan ssh.NewChannel) {
	for ch := range chans {
		epdJSON := ch.ExtraData()
		var epd chshare.ChannelEndpointDescriptor
		err := json.Unmarshal(epdJSON, &epd)
		if err != nil {
			c.Debugf("Error: Remote channel connect request: bad JSON parameter string: '%s'", epdJSON)
			ch.Reject(ssh.UnknownChannelType, "Bad JSON ExtraData")
			continue
		}
		c.Debugf("Remote channel connect request, endpoint ='%s'", epd.LongString())
		if epd.Role != chshare.ChannelEndpointRoleSkeleton {
			c.Debugf("Error: Remote channel connect request: Role must be skeleton: '%s'", epd.LongString())
			ch.Reject(ssh.Prohibited, "Endpoint role must be skeleton")
			continue
		}
		if epd.Type == chshare.ChannelEndpointTypeStdio {
			c.Debugf("Error: Remote channel connect request: Client-side skeleton STDIO not yet supported: '%s'", epd.LongString())
			ch.Reject(ssh.Prohibited, "Client-side skeleton STDIO not yet supported")
			continue
		}
		if epd.Type == chshare.ChannelEndpointTypeLoop {
			c.Debugf("Error: Remote channel connect request: Loop channels not yet not supported: '%s'", epd.LongString())
			ch.Reject(ssh.Prohibited, "Loop channels not yet supported")
			continue
		}
		if epd.Type == chshare.ChannelEndpointTypeUnix {
			c.Debugf("Error: Remote channel connect request: Unix domain sockets not yet not supported: '%s'", epd.LongString())
			ch.Reject(ssh.Prohibited, "Unix domain sockets not yet supported")
			continue
		}
		if epd.Type == chshare.ChannelEndpointTypeSocks {
			c.Debugf("Error: Remote channel connect request: client-side SOCKS endpoint not yet not supported: '%s'", epd.LongString())
			ch.Reject(ssh.Prohibited, "Client-side SOCKS endpoint not yet supported")
			continue
		}

		// TODO: The actual local connect request should succeed before we accept the remote request.
		//       Need to refactor code here
		stream, reqs, err := ch.Accept()
		if err != nil {
			c.Debugf("Failed to accept remote stream: %s", err)
			continue
		}
		go ssh.DiscardRequests(reqs)
		//handle stream type
		l := c.Logger.Fork("conn#%d", c.connStats.New())
		go chshare.HandleTCPStream(l, &c.connStats, stream, epd.Path)
	}
}
