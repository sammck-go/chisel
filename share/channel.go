package chshare

import (
	"fmt"
	// "net/url"
	"io"
	"regexp"
	"strconv"
	"strings"
	// "github.com/golang-collections/collections/stack"
)

// To distinguish between the various entities in a tunneled
// network environment, we adopt the vocabulary of distributed object
// communication, in particular "Stub" for the proxy's impersonation
// of a network service, and "Skeleton" for the proxy's impersonation
// of a network client.
//   (see https://en.wikipedia.org/wiki/Distributed_object_communication).
//
// A "Chisel Proxy" is an instance of this chisel application, running as either a
// "Chisel Proxy Client" or a "Chisel Proxy Server". A "Chisel Proxy Session" consists
// of a single Chisel Proxy Client and a single Chisel Proxy Server, communicating with
// one another over a single TCP connection using a encrypted SSH protocol over WebSockets.
// A Chisel Proxy Client participates in exactly on Chisel Proxy Session, but a Chisel Proxy
// Server may have many concurrent Chisel Proxy Sessions with different Chisel Proxy Clients.
// All traffic through a Chisel Proxy Session, including proxied network service
// traffic, is encrypted with the SSH protocol. Provided the private key of the Chisel Proxy
// Server is kept private, and the public key fingerprint of the Chisel Proxy Server is known
// in advance by all Chisel Proxy Clients, there is no need for additional encryption of
// proxied traffic (it may still be desirable to encrypt proxied traffic between Chisel Proxies and
// the applications that connect to the proxy).
//
// A Local Chisel Proxy is the Chisel Proxy that is directly network-reachable in the context
// of a particular discussion. It may be either a Chisel Proxy Client or a Chisel Proxy Server.
//
// A Remote Chisel Proxy is the other Chisel Proxy in the same Chisel Proxy Session as a
// given Local Chisel Proxy.
//
// A "Caller" is an originating application that wishes to connect to a logical
// network "Called Service" that can not be reached locally, but which must be reached
// through the Chisel Proxy Session. Typically this is a TCP client, though other protocols
// are supported.
//
// A "Called Service" is a logical network service that needs to be accessed by
// "Callers" that cannot reach it locally, but must reach it through a Chisel Proxy Session.
// Typically this is a TCP service listening at a particular host/port, though other
// protocols are supported.
//
// A ChannelEndpoint is a logical network endpoint on a Local Chisel Proxy that
// is paired with another ChannelEndpoint on the Remote Chisel Proxy.
// A ChannelEndpoint is either a "Stub", meaning that it impersonates a remote Called Service
// and listens for and accepts connection requests from local Callers and forwards them
// to the peered ChannelEndpoint on the Remote Chisel Proxy, or a Skeleton, meaning that it responds to
// connection requests from the Remote Chisel Proxy and impersonates the remote Caller
// by connecting to locally available Called Services.
//
// A Stub looks like a Called Service to a Caller, and a Skeleton looks like
// a Caller to a Called Service.
//
// We will refer to the combination of a Stub ChannelEndpoint and its peered remote
// Skeleton ChannelEndpoint as a "Channel Endpoint Pair". Since it is a distributed
// entity, a Channel Endpoint Pair has no direct representation in code. A Channel Endpoint
// Pair in which Stub ChannelEndpoint is on the Chisel Proxy Client is referred to
// as operating in "forward proxy mode". A Channel Endpoint Pair in which Stub
// ChannelEndpoint is on the Chisel Proxy Server is referred to as operating in
// "reverse proxy mode".
//
// Just as a socket service listener can accept and service multiple concurrent
// connections from clients, a Channel Endpoint Pair can accept and service multiple
// concurrent connections from clients at the Stub side, and proxy them to multiple
// concurrent service connections at the Skeleton side. Each individual proxied
// connection of this type is called a Channel. Traffic on each Channel is completely
// independent of and asynchronous to traffic on other Channels. all Channel
// traffic is multiplexed through a Chisel Proxy Session on the single TCP connection
// used by the Chisel Proxy Session; flow control is employed to ensure that a delay
// in reading from one Channel has no effect on the availability of other channels or on
// traffic in the opposite direction on the same channel.
//
// A ChannelEndpoint is described in a serializable form by a ChannelEndpointDescriptor.
//
// A Channel Endpoint Pair is described in serialized form by a ChannelDescriptor.
//
// One type of ChannelEndpoint that deserves special mention is a "Loop" ChannelEndpoint.
// A Loop Stub ChannelEndpoint operates very much like a Unix Domain Socket Stub (it
// listens on a specified name in a string-based namespace), except that it can only
// accept connections from Loop Skeleton ChannelEndpoints on the Same Chisel Proxy (it has
// no visibility outside of the Chisel Proxy). The advantage of Loop Channels is that
// traffic is directly forwarded between a Caller's Channel and a Called service's
// Channel, when both the Caller and the Called Service are reachable only through
// a Chisel Proxy Session. This effectively saves two network hops for traffic
// in both directions (writing to a unix domain socket and reading back from the
// same unix domain socket)
//
//                           +-----------------------------+             +--------------------------------+
//                           |     Chisel Proxy Client     |    Chisel   |     Chisel Proxy Server        |
//                           |                             |   Session   |                                |
//                           |                             | ==========> |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |      | Server-side    |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton          | =====|====> | Called Service |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |      |      | Server-side    |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub              | <====|===== | Caller App     |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton "Loop"   | ===\ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |    | |
//   +----------------+      |    +-------------------+    |             |    +--------------------+    | |
//                           |                             |             |                              | |
//                           +-----------------------------|             |                              | |
//                                                                       |                              | |
//                           +-----------------------------+             |                              | |
//                           |     Chisel Proxy Client     |    Chisel   |                              | |
//                           |                             |   Session   |                              | |
//                           |                             | ==========> |                              | |
//   +----------------+      |    +-------------------|    |             |    +--------------------+    | |
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |    | |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub "Loop"       | <==/ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |
//                           |                             |             |                                |
//                           +-----------------------------+             +--------------------------------+
//

// ChannelEndpointRole defines whether an endpoint is acting as
// the Stub or the Skeleton for al ChannelEndpointPair
type ChannelEndpointRole string

const (
	// ChannelEndpointRoleUnknown is an unknown (uninitialized) endpoint role
	ChannelEndpointRoleUnknown ChannelEndpointRole = "unknown"

	// ChannelEndpointRoleStub is associated with an endpoint whose chisel
	// instance must listen for and accept new connections from local clients,
	// forward connection requests to the remote proxy where services
	// are locally available. An Stub endpoint may accept multiple
	// connection requests from local clients, resulting in multiple concurrent
	// tunnelled connections to the remote Skeleton endpoint proxy.
	ChannelEndpointRoleStub ChannelEndpointRole = "stub"

	// ChannelEndpointRoleSkeleton is associated with an endpoint whose chisel
	// instance must accept connection requests from the remote proxy
	// and actively reach out and connect to locally available services.
	// A Skeleton endpoint may accept multiple connection requests from the
	// remote proxy, resulting in multiple concurrent socket connections to
	// locally avaiable services
	ChannelEndpointRoleSkeleton ChannelEndpointRole = "skeleton"
)

// ChannelEndpointType describes the protocol used for a particular endpoint
type ChannelEndpointType string

const (
	// ChannelEndpointTypeUnknown is an unknown (uninitialized) endpoint type
	ChannelEndpointTypeUnknown ChannelEndpointType = "unknown"

	// ChannelEndpointTypeTCP is a TCP endpoint--either a host/port for Skeleton or
	//  a local bind address/port for Stub
	ChannelEndpointTypeTCP ChannelEndpointType = "tcp"

	// ChannelEndpointTypeUnix is a Unix Domain Socket (AKA local socket) endpoint, identified
	// by filesystem pathname, for either a Skeleton or Stub.
	ChannelEndpointTypeUnix ChannelEndpointType = "unix"

	// ChannelEndpointTypeSocks is a logical SOCKS server. Only meaningful for Skeleton. When
	// a connection request is received from the remote proxy (on behalf of the respective Stub),
	// it is connected to an internal SOCKS server.
	ChannelEndpointTypeSocks ChannelEndpointType = "socks"

	// ChannelEndpointTypeStdio is a preconnected virtual socket connected to its proxy process's stdin
	// and stdout. For an Stub, the connection is established and forwarded to the remote proxy
	// immediately after the remote proxy session is initialized. For a Skeleton, the connection is
	// considered active as soon as a connect request is received from the remote proxy service. This type of
	// endpoint can only be associated with the proxy client's end, can only be connected once,
	// and once that connection is closed, it can no longer be used or reconnected for the duration
	// of the session with the remote proxy. There can only be one Stdio endpoint defined on a given
	// proxy client.
	ChannelEndpointTypeStdio ChannelEndpointType = "stdio"

	// ChannelEndpointTypeLoop ChannelEndpointType is a virtual loopack socket, identified by an opaque
	// endpoint name. It is similar to a Unix domain socket in that it is identified by a unique
	// string name to which both a Caller and a Called Service ultimately bind. However, The name is only
	// resovable within a particular Chisel Proxy, and a Stub endpoint of this type can only be reached
	// by a Skeleton endpoint of this type on the same Chisel Proxy. Traffic on Channels of this type is
	// directly forwarded between the Stub and the Skeleton on the Chisel Proxy server, eliminating two
	// open os socket handles and two extra socket hops that would be required if ordinary sockets were used.
	ChannelEndpointTypeLoop ChannelEndpointType = "loop"
)

// ChannelEndpointDescriptor describes one end of a ChannelDescriptor
type ChannelEndpointDescriptor struct {
	// Which end of the tunnel pair this endpoint occupies (Stub or Skeleton)
	Role ChannelEndpointRole `json:"role"`

	// What type of endpoint is this (e.g., TCP, unix domain socket, stdio, etc...)
	Type ChannelEndpointType `json:"type"`

	// The "name" associated with the endpoint. This depends on role and type:
	//
	//     TYPE    ROLE        PATH
	//     TCP     Stub        <local-ipv4-bind-address>:<port> for listen
	//     TCP     Skeleton    <hostname>:<port> for connect
	//     Unix    Stub        <Filesystem path of domain socket> for listen
	//     Unix    Skeleton    <Filesystem path of domain socket> for connect
	//     SOCKS   Skeleton    nil
	//     Stdio   Stub        nil
	//     Stdio   Skeleton    nil
	//     Loop    Stub        <loop-endpoint-name> for listen
	//     Loop    Skeleton    <loop-endpoint-name> for connect
	Path string `json:"path"`
}

func (d ChannelEndpointDescriptor) Validate() error {
	if d.Role != ChannelEndpointRoleStub && d.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Unknown role type '%s'", d.String(), d.Role)
	}
	if d.Type == ChannelEndpointTypeTCP {
		if d.Path == "" {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint requires a bind address and port", d.String())
			} else {
				return fmt.Errorf("%s: TCP skeleton endpoint requires a target hostname and port", d.String())
			}
		} else {
			_, port, err := ParseHostPort(d.Path, InvalidPort)
			if err != nil {
				if d.Role == ChannelEndpointRoleStub {
					return fmt.Errorf("%s: TCP stub endpoint <bind-address>:<port> is invalid: %v", d.String(), err)
				} else {
					return fmt.Errorf("%s: TCP skeleton endpoint <hostname>:<port> is invalied: %v", d.String(), err)
				}
			}
			if port == InvalidPort {
				return fmt.Errorf("%s: TCP endpoint requires a port number", d.String())
			}
		}
	} else if d.Type == ChannelEndpointTypeUnix {
		if d.Path == "" {
			return fmt.Errorf("%s: Unix domain socket endpoint requires a socket pathname", d.String())
		}
	} else if d.Type == ChannelEndpointTypeLoop {
		if d.Path == "" {
			return fmt.Errorf("%s: Loop endpoint requires a loop name", d.String())
		}
	} else if d.Type == ChannelEndpointTypeStdio {
		if d.Path != "" {
			return fmt.Errorf("%s: STDIO endpoint cannot have a path", d.String())
		}
	} else if d.Type == ChannelEndpointTypeSocks {
		if d.Path != "" {
			return fmt.Errorf("%s: SOCKS endpoint cannot have a path", d.String())
		}
		if d.Role != ChannelEndpointRoleSkeleton {
			return fmt.Errorf("%s: SOCKS endpoint must be placed on the skeleton side", d.String())
		}
	} else {
		return fmt.Errorf("%s: Unknown endpoint type '%s'", d.String(), d.Type)
	}
	return nil
}

func (d ChannelEndpointDescriptor) String() string {
	typeName := string(d.Type)
	if typeName == "" {
		typeName = "unknown"
	}
	pathName := d.Path
	return "<" + typeName + ":" + pathName + ">"
}

func (d ChannelEndpointDescriptor) LongString() string {
	roleName := string(d.Role)
	if roleName == "" {
		roleName = "unknown"
	}
	typeName := string(d.Type)
	if typeName == "" {
		typeName = "unknown"
	}

	return "ChannelEndpointDescriptor(role='" + roleName + "', type='" + typeName + "', path='" + d.Path + "')"
}

// ChannelDescriptor describes a pair of endpoints, one on the client proxy and one
// on the server proxy, which together define a single tunnelled socket service.
type ChannelDescriptor struct {
	// Reverse: if true, indicates that this is a reverse-proxy tunnel--i.e., the
	// Stub is on the server proxy and the listener is on the client proxy.
	// If False, the answerer is on the client proxy and the Skeleton is on the
	// server proxy.
	Reverse bool

	// Stub is the endpoint that listens for and accepts local connections, and forwards
	// them to the remote proxy. Ordinarily the Stub is on the client proxy, but this
	// is flipped if Reverse==true.
	Stub ChannelEndpointDescriptor

	// Skeleton is the endpoint that receives connect requests from the remote proxy
	// and forwards them to locally accessible network services. Ordinarily the
	// Skeleton is on the server proxy, but this is flipped if Reverse==true.
	Skeleton ChannelEndpointDescriptor
}

func (d ChannelDescriptor) Validate() error {
	err := d.Stub.Validate()
	if err != nil {
		return err
	}
	err = d.Skeleton.Validate()
	if err != nil {
		return err
	}
	if d.Stub.Role != ChannelEndpointRoleStub {
		return fmt.Errorf("%s: Role of stub must be ChannelEndpointRoleStub", d.String())
	}
	if d.Skeleton.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Role of skeleton must be ChannelEndpointRoleSkeleton", d.String())
	}

	if (!d.Reverse && d.Skeleton.Type == ChannelEndpointTypeStdio) ||
		(d.Reverse && d.Stub.Type == ChannelEndpointTypeStdio) {
		return fmt.Errorf("%s: STDIO endpoint must be on client proxy side", d.String())
	}

	return nil
}

func (d ChannelDescriptor) String() string {
	reversePrefix := ""
	if d.Reverse {
		reversePrefix = "R:"
	}
	return reversePrefix + d.Stub.String() + ":" + d.Skeleton.String()
}

func (d ChannelDescriptor) LongString() string {
	reverseStr := "false"
	if d.Reverse {
		reverseStr = "true"
	}

	return "ChannelDescriptor(reverse='" + reverseStr + "', stub=" + d.Stub.LongString() + ", skeleton=" + d.Skeleton.LongString() + ")"
}

// Fully qualified ChannelDescriptor
//    ["R:"]<stub-type>:<stub-path>:<skeleton-type>:<skeleton-path>
//
// Where the optional "R:" prefix indicates a reverse-proxy
//   <stub-type> is one of TCP, UNIX, STDIO, or LOOP.
//   <skeleton-type> is one of: TCP, UNIX, SOCKS, STDIO, or LOOP
//   <stub-path> and <skeleton-path> are formatted according to respective type:
//        stub TCP:        <IPV4 bind addr>:<port>                          0.0.0.0:22
//                         [<IPV6 bind addr>]:<port>                        0.0.0.0:22
//        skeleton TCP:
//
// Note that any ":"-delimited descriptor element that contains a ":" may be escaped in the following ways:
//    * Except as indicated below, the presence of '[' or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    * An element that begins and ends with '<' and a balanced '>' will have the beginning and ending characters
//        stripped off of the final parsed element
//    '\:' will be a converted to a single ':' within an element but will not be recognized as a delimiter
//    '\\' will be converted to a single '\' within an element
//    '\<' Will be converted to a single '<' and will not be considered for bracket balancing
//    '\>' will be converted to a single '>' and will not be considered for bracket balancing
//    '\[' Will be converted to a single '[' and will not be considered for bracket balancing
//    '\]' will be converted to a single ']' and will not be considered for bracket balancing
//
//
// Short-hand conversions
//   3000 ->
//     local  127.0.0.1:3000
//     remote 127.0.0.1:3000
//   foobar.com:3000 ->
//     local  127.0.0.1:3000
//     remote foobar.com:3000
//   3000:google.com:80 ->
//     local  127.0.0.1:3000
//     remote google.com:80
//   192.168.0.1:3000:google.com:80 ->
//     local  192.168.0.1:3000
//     remote google.com:80

type bracketStack struct {
	runes []rune
	n     int
}

func (s *bracketStack) pushBracket(r rune) {
	s.runes = append(s.runes[:s.n], r)
	s.n++
}

func (s *bracketStack) popBracket() rune {
	var c rune
	if s.n > 0 {
		s.n--
		c = s.runes[s.n]
	}
	return c
}

func (s *bracketStack) isBalanced() bool {
	return s.n == 0
}

// SplitChannelDescriptorParts breaks a ":"-delimited channel descriptor string
// into its parts, respecting the various escaping mechanisms described above.
func SplitChannelDescriptorParts(s string) ([]string, error) {

	bStack := &bracketStack{}

	var result []string
	partial := ""
	haveBackslash := false
	stripAngleBrackets := false
	startingElement := true
	closeToOpen := map[rune]rune{
		'>': '<',
		']': '[',
	}

	flushPartial := func(final bool) error {
		if !bStack.isBalanced() {
			return fmt.Errorf("SplitChannelDescriptorParts: unmatched '%c' in descriptor '%s'", bStack.popBracket(), s)
		}
		if haveBackslash {
			return fmt.Errorf("SplitChannelDescriptorParts: descriptor ends in backslash: '%s'", s)
		}
		if !(final && len(partial) == 0 && len(result) == 0) {
			if stripAngleBrackets {
				partial = partial[1 : len(partial)-1]
				stripAngleBrackets = false
			}
			result = append(result, partial)
			partial = ""
		}
		return nil
	}

	for _, c := range s {
		wasStartingElement := startingElement
		startingElement = false
		if haveBackslash {
			partial += string(c)
			haveBackslash = false
		} else if c == '\\' {
			haveBackslash = true
		} else if c == '[' {
			partial += string(c)
			bStack.pushBracket('[')
		} else if c == '<' {
			partial += string(c)
			bStack.pushBracket('<')
			if wasStartingElement {
				stripAngleBrackets = true
			}
		} else if c == '>' || c == ']' {
			if bStack.isBalanced() {
				return nil, fmt.Errorf("SplitChannelDescriptorParts: unmatched '%c' in descriptor '%s'", c, s)
			}
			actualOpen := bStack.popBracket()
			expectedOpen := closeToOpen[c]
			if actualOpen != expectedOpen {
				return nil, fmt.Errorf(
					"SplitChannelDescriptorParts: mismatched bracket types, "+
						"opened with '%c', closed with '%c' in descriptor '%s'", actualOpen, c, s)
			}
			partial += string(c)
		} else if c == ':' && bStack.isBalanced() {
			err := flushPartial(false)
			if err != nil {
				return nil, err
			}
			startingElement = true
		} else {
			partial += string(c)
		}
	}
	err := flushPartial(true)
	if err != nil {
		return nil, err
	}

	return result, nil
}

const revPrefix = "R"

const InvalidPort = -1

var isPortRegExp = regexp.MustCompile(`^\d+$`)

func isPort(s string) bool {
	if !isPortRegExp.MatchString(s) {
		return false
	}
	return true
}

// ParseHostPort breaks a <hostname>:<port>, <bind-addr>:<port>,
// <hostname>, or <bind-addr> into a tuple.
func ParseHostPort(path string, defaultPort int) (string, int, error) {
	lastColonPos := strings.LastIndex(path, ":")
	if lastColonPos >= 0 {
		lastBracketPos := strings.LastIndex(path, "]")
		if lastBracketPos < lastColonPos {
			host := path[:lastColonPos]
			portStr := path[lastColonPos+1:]
			portU64, err := strconv.ParseUint(portStr, 10, 16)
			if err != nil {
				return "", InvalidPort, err
			}
			return host, int(portU64), nil
		}
	}
	return path, defaultPort, nil
}

// ParseChannelDescriptor parses a string representing a ChannelDescriptor
func ParseChannelDescriptor(s string) (*ChannelDescriptor, error) {
	reverse := false
	parts, err := SplitChannelDescriptorParts(s)
	if err != nil {
		return nil, err
	}
	if len(parts) > 0 && parts[0] == "R" {
		reverse = true
		parts = parts[1:]
	}
	if len(parts) <= 0 {
		return nil, fmt.Errorf("Empty channel descriptor string not allowed: '%s'", s)
	}

	d := &ChannelDescriptor{
		Reverse: reverse,
		Stub: ChannelEndpointDescriptor{
			Role: ChannelEndpointRoleStub,
		},
		Skeleton: ChannelEndpointDescriptor{
			Role: ChannelEndpointRoleSkeleton,
		},
	}
	haveStub := false
	haveSkeleton := false
	haveType := false
	havePath := false
	havePort := false
	var currentType ChannelEndpointType
	var currentPath string

	flushPartialEndpoint := func() error {
		if haveType {
			if haveSkeleton {
				return fmt.Errorf("Too many elements in channel descriptor string: '%s'", s)
			}
			if !haveStub {
				d.Stub.Type = currentType
				d.Stub.Path = currentPath
				haveStub = true
			} else {
				d.Skeleton.Type = currentType
				d.Skeleton.Path = currentPath
				haveSkeleton = true
			}
			haveType = false
			havePath = false
			havePort = false
			currentType = ChannelEndpointTypeUnknown
			currentPath = ""
		}
		return nil
	}

	for _, p := range parts {
		if p == "stdio" {
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
			currentType = ChannelEndpointTypeStdio
			haveType = true
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
		} else if p == "socks" {
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
			currentType = ChannelEndpointTypeSocks
			haveType = true
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
		} else if p == "tcp" {
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
			currentType = ChannelEndpointTypeTCP
			haveType = true
		} else if p == "unix" {
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
			currentType = ChannelEndpointTypeUnix
			haveType = true
		} else if p == "loop" {
			err = flushPartialEndpoint()
			if err != nil {
				return nil, err
			}
			currentType = ChannelEndpointTypeLoop
			haveType = true
		} else if isPort(p) {
			if haveType && (currentType != ChannelEndpointTypeTCP || havePort) {
				err = flushPartialEndpoint()
				if err != nil {
					return nil, err
				}
			}
			currentType = ChannelEndpointTypeTCP
			haveType = true
			if !havePath {
				currentPath = ""
				havePath = true
			}
			currentPath = currentPath + ":" + p
			havePort = true
		} else {
			if havePath {
				err = flushPartialEndpoint()
				if err != nil {
					return nil, err
				}
			}
			if !haveType {
				if strings.HasPrefix(p, "tcp:") {
					currentType = ChannelEndpointTypeTCP
					p = p[4:]
				} else if strings.HasPrefix(p, "unix:") {
					currentType = ChannelEndpointTypeUnix
					p = p[5:]
				} else if strings.HasPrefix(p, "loop:") {
					currentType = ChannelEndpointTypeLoop
					p = p[5:]
				} else if strings.HasPrefix(p, "socks:") {
					currentType = ChannelEndpointTypeLoop
					p = p[6:]
				} else if strings.HasPrefix(p, "stdio:") {
					currentType = ChannelEndpointTypeStdio
					p = p[6:]
				} else if strings.HasPrefix(p, "/") || strings.HasPrefix(p, ".") {
					currentType = ChannelEndpointTypeUnix
				} else {
					currentType = ChannelEndpointTypeTCP
				}
				haveType = true
			}
			currentPath = p
			havePath = true
		}
	}

	err = flushPartialEndpoint()
	if err != nil {
		return nil, err
	}

	if d.Stub.Type == ChannelEndpointTypeUnknown {
		return nil, fmt.Errorf("Insufficient information in channel descriptor string: '%s'", s)
	}

	if d.Stub.Type == ChannelEndpointTypeSocks {
		// Special case, allow *only* specifying socks, in which case move it from the Stub to the
		// Skeleton where it belongs
		if d.Skeleton.Type == ChannelEndpointTypeUnknown {
			tmp := d.Stub
			d.Skeleton = d.Stub
			d.Stub = tmp
			d.Stub.Role = ChannelEndpointRoleStub
			d.Skeleton.Role = ChannelEndpointRoleSkeleton
		}
		if d.Stub.Type == ChannelEndpointTypeUnknown {
			d.Stub.Type = ChannelEndpointTypeTCP
		}
	}

	if d.Stub.Type == ChannelEndpointTypeSocks {
		return nil, fmt.Errorf("SOCKS endpoints are only allowed on the skeleton side: '%s'", s)
	}

	if d.Skeleton.Type == ChannelEndpointTypeUnknown {
		d.Skeleton.Type = ChannelEndpointTypeTCP
	}

	stubBindAddr := ""
	stubPort := InvalidPort
	skeletonHost := ""
	skeletonPort := InvalidPort

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if len(d.Stub.Path) > 0 {
			stubBindAddr, stubPort, err = ParseHostPort(d.Stub.Path, InvalidPort)
			if err != nil {
				return nil, err
			}
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if len(d.Skeleton.Path) > 0 {
			skeletonHost, skeletonPort, err = ParseHostPort(d.Skeleton.Path, InvalidPort)
			if err != nil {
				return nil, err
			}
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP && stubBindAddr == "" {
		if d.Skeleton.Type == ChannelEndpointTypeSocks {
			stubBindAddr = "127.0.0.1"
		} else {
			stubBindAddr = "0.0.0.0"
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP && stubPort == InvalidPort {
		if d.Skeleton.Type == ChannelEndpointTypeSocks {
			stubPort = 1080
		} else if skeletonPort != InvalidPort {
			stubPort = skeletonPort
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP && skeletonPort == InvalidPort {
		if stubPort != InvalidPort {
			skeletonPort = stubPort
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if stubBindAddr == "" {
			return nil, fmt.Errorf("Unable to determine stub bind address in channel descriptor string: '%s'", s)
		}
		if stubPort == InvalidPort {
			return nil, fmt.Errorf("Unable to determine stub port number in channel descriptor string: '%s'", s)
		}

		d.Stub.Path = stubBindAddr + ":" + strconv.Itoa(stubPort)
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if skeletonHost == "" {
			return nil, fmt.Errorf("Unable to determine skeleton host name in channel descriptor string: '%s'", s)
		}
		if skeletonPort == InvalidPort {
			return nil, fmt.Errorf("Unable to determine skeleton port number in channel descriptor string: '%s'", s)
		}
		d.Skeleton.Path = skeletonHost + ":" + strconv.Itoa(skeletonPort)
	}

	if (d.Stub.Type == ChannelEndpointTypeStdio && d.Reverse) || (d.Stub.Type == ChannelEndpointTypeStdio && d.Reverse) {
		return nil, fmt.Errorf("Stdio endpoints are only allowed on the client proxy side: '%s'", s)
	}

	if d.Skeleton.Type == ChannelEndpointTypeUnknown {
		return nil, fmt.Errorf("Unable to determine skeleton endpoint type: '%s'", s)
	}

	err = d.Validate()
	if err != nil {
		return nil, err
	}

	return d, nil
}

type Channel interface {
	io.ReadWriteCloser
	GetLocalEndpointDescriptor()
}

type ChannelConn interface {
	io.ReadWriteCloser
}

type LocalChannelConn interface {
	ChannelConn
}

type ChannelEndpoint interface {
	io.Closer
	GetChannelEndpointDescriptor() ChannelEndpointDescriptor
}

type StubChannelEndpoint interface {
	ChannelEndpoint
	Accept() (LocalChannelConn, error)
}

type SkeletonChannelEndpoint interface {
	ChannelEndpoint
	Dial() (LocalChannelConn, error)
}
