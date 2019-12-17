package chshare

import (
	"context"
	"fmt"
	socks5 "github.com/armon/go-socks5"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// Validate a ChannelEndpointDescriptor
func (d ChannelEndpointDescriptor) Validate() error {
	if d.Role != ChannelEndpointRoleStub && d.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Unknown role type '%s'", d.String(), d.Role)
	}
	if d.Type == ChannelEndpointTypeTCP {
		if d.Path == "" {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint requires a bind address and port", d.String())
			}
			return fmt.Errorf("%s: TCP skeleton endpoint requires a target hostname and port", d.String())
		}
		host, port, err := ParseHostPort(d.Path, "", InvalidPortNumber)
		if err != nil {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint <bind-address>:<port> is invalid: %v", d.String(), err)
			}
			return fmt.Errorf("%s: TCP skeleton endpoint <hostname>:<port> is invalid: %v", d.String(), err)
		}
		if host == "" {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint requires a bind address: %v", d.String(), err)
			}
			return fmt.Errorf("%s: TCP skeleton endpoint requires a target hostname: %v", d.String(), err)
		}
		if port == InvalidPortNumber {
			return fmt.Errorf("%s: TCP endpoint requires a port number", d.String())
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

// LongString converts a ChannelEndpointDescriptor to a long descriptive string
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
	Stub *ChannelEndpointDescriptor

	// Skeleton is the endpoint that receives connect requests from the remote proxy
	// and forwards them to locally accessible network services. Ordinarily the
	// Skeleton is on the server proxy, but this is flipped if Reverse==true.
	Skeleton *ChannelEndpointDescriptor
}

// Validate a ChannelDescriptor
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

// LongString converts a ChannelDescriptor to a long descriptive string
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

// SplitBracketedParts breaks a ":"-delimited channel descriptor string
// into its parts, respecting the following escaping mechanisms:
//
//    * Except as indicated below, the presence of '[' or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    '\:' will be a converted to a single ':' within an element but will not be recognized as a delimiter
//    '\\' will be converted to a single '\' within an element
//    '\<' Will be converted to a single '<' and will not be considered for bracket balancing
//    '\>' will be converted to a single '>' and will not be considered for bracket balancing
//    '\[' Will be converted to a single '[' and will not be considered for bracket balancing
//    '\]' will be converted to a single ']' and will not be considered for bracket balancing
func SplitBracketedParts(s string) ([]string, error) {
	bStack := &bracketStack{}

	var result []string
	partial := ""
	haveBackslash := false
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
			result = append(result, partial)
			partial = ""
		}
		return nil
	}

	for _, c := range s {
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

// PortNumber is a TCP port number in the range 0-65535. 0 is defined as UnknownPortNumber
// and 65535 is defined as InvalidPortNumber
type PortNumber uint16

// UnknownPortNumber is an unknown TCP port number. The zero value for PortNumber
const UnknownPortNumber PortNumber = 0

// InvalidPortNumber is an invalid TCP port number. Equal to uint16(65535)
const InvalidPortNumber PortNumber = 65535

// ParsePortNumber converts a string to a PortNumber
//   An error will be returned if the string is not a valid integer in the range
//   1-65534. If the string is 0, UnknownPortNumber will be returned as the
//   value. All other error conditionss will return InvalidPortNumber as the value.
func ParsePortNumber(s string) (PortNumber, error) {
	p64, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return InvalidPortNumber, fmt.Errorf("Invalid port number %s: %s", s, err)
	}
	p := PortNumber(uint16(p64))
	if p == InvalidPortNumber {
		err = fmt.Errorf("65535 is a reserved invalid port number")
	} else if p == UnknownPortNumber {
		err = fmt.Errorf("0 is a reserved unknown port number")
	}
	return p, err
}

func (x PortNumber) String() string {
	var result string
	if x == InvalidPortNumber {
		result = "<invalid>"
	} else if x == UnknownPortNumber {
		result = "<unknown>"
	} else {
		result = strconv.FormatUint(uint64(x), 10)
	}
	return result
}

// IsPortNumberString returns true if the string can be parsed into a valid TCP PortNumber
func IsPortNumberString(s string) bool {
	_, err := ParsePortNumber(s)
	return err == nil
}

func isAngleBracketed(s string) bool {
	if len(s) < 2 || s[0] != '<' || s[len(s)-1] != '>' {
		return false
	}

	bStack := &bracketStack{}

	haveBackslash := false
	closePos := -1

	for i, c := range s {
		if haveBackslash {
			haveBackslash = false
		} else if c == '\\' {
			haveBackslash = true
		} else if c == '<' {
			bStack.pushBracket('<')
		} else if c == '>' || c == ']' {
			bStack.popBracket()
			if bStack.isBalanced() {
				closePos = i
				break
			}
		}
	}

	return closePos == len(s)-1
}

// StripAngleBrackets removes balanced leading and trailing '<' and '>' pair on a string, if they are present
func StripAngleBrackets(s string) string {
	if isAngleBracketed(s) {
		s = s[1 : len(s)-1]
	}
	return s
}

// ParseHostPort breaks a <hostname>:<port>, <hostname>, or <port> into a tuple.
//   <hostname> may contain balanced square or angle brackets, inside which ':'
//   characters are not considered as a delimiter. This allows for IPV6 host/port
//   specification such as [2001:0000:3238:DFE1:0063:0000:0000:FEFB]:80
//   In addition the entire path or the host, (but not the port) may be enclosed in
//   angle brackets, which will be stripped.
func ParseHostPort(path string, defaultHost string, defaultPort PortNumber) (string, PortNumber, error) {
	var port PortNumber
	var host string

	bpath := StripAngleBrackets(path)

	parts, err := SplitBracketedParts(bpath)
	if err != nil {
		return "", InvalidPortNumber, fmt.Errorf("Invalid TCP host/port string: %s: %s", err, path)
	}

	if len(parts) > 2 {
		return "", InvalidPortNumber, fmt.Errorf("Too many ':'-delimited parts in TCP host/port string: %s", path)
	} else if len(parts) == 1 {
		part := parts[0]
		port, err = ParsePortNumber(part)
		if err != nil {
			port = UnknownPortNumber
			host = StripAngleBrackets(part)
		}
	} else if len(parts) == 2 {
		host = StripAngleBrackets(parts[0])
		port, err = ParsePortNumber(parts[1])
		if err != nil {
			return "", InvalidPortNumber, fmt.Errorf("Invalid port in TCP host/port string: %s: %s", err, path)
		}
	}

	if host == "" {
		host = defaultHost
	}

	if port == UnknownPortNumber {
		port = defaultPort
	}

	return host, port, nil
}

// ParseNextChannelEndpointDescriptor parses the next ChannelEndpointDescriptor out of a presplit ":"-delimited string,
// returning the remainder of unparsed parts
func ParseNextChannelEndpointDescriptor(parts []string, role ChannelEndpointRole) (*ChannelEndpointDescriptor, []string, error) {
	s := strings.Join(parts, ":")
	if len(parts) <= 0 {
		return nil, parts, fmt.Errorf("Empty endpoint descriptor string not allowed: '%s'", s)
	}

	d := &ChannelEndpointDescriptor{Role: role}

	haveType := false
	havePath := false
	lastI := len(parts) - 1

	for i, p := range parts {
		sp := StripAngleBrackets(p)
		if sp == "stdio" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeStdio
			lastI = i
			break
		} else if sp == "socks" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeSocks
			lastI = i
			break
		} else if sp == "tcp" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeTCP
			haveType = true
		} else if sp == "unix" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeUnix
			haveType = true
		} else if sp == "loop" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeUnix
			haveType = true
		} else if IsPortNumberString(sp) {
			if haveType && d.Type != ChannelEndpointTypeTCP {
				break
			}
			d.Type = ChannelEndpointTypeTCP
			port, _ := ParsePortNumber(sp)
			d.Path = d.Path + ":" + port.String()
			lastI = i
			break
		} else {
			// Not an endpoint type name or a port number. Either
			//  1: An angle-bracketed endpoint specifier
			//  2: A path associated with an already parsed edpoint type
			//  3: A path with an implicit endpoint type (tcp or unix)
			if havePath {
				lastI = i
				break
			}
			if !haveType {
				spParts, err := SplitBracketedParts(sp)
				if err != nil {
					return nil, parts, fmt.Errorf("Invalid endpoint descriptor string '%s': '%s'", s, err)
				}
				if len(spParts) > 1 {
					// This must be an angle-bracketed standalone endpoint descriptor, so we will recurse
					d, err = ParseChannelEndpointDescriptor(sp, role)
					if err != nil {
						return nil, parts, err
					}
					lastI = i
					break
				}

				var spp0 string
				if len(spParts) > 0 {
					spp0 = StripAngleBrackets(spParts[0])
				}

				if spp0 == "stdio" {
					d.Type = ChannelEndpointTypeStdio
					lastI = i
					break
				}

				if spp0 == "socks" {
					d.Type = ChannelEndpointTypeSocks
					lastI = i
					break
				}

				if strings.HasPrefix(spp0, "/") || strings.HasPrefix(spp0, ".") {
					d.Type = ChannelEndpointTypeUnix
					d.Path = spp0
					lastI = i
					break
				}

				d.Type = ChannelEndpointTypeTCP
				d.Path = spp0
				haveType = true
				havePath = true
			} else {
				// a path to go with explicitly provided endpoint type
				if d.Type != ChannelEndpointTypeTCP {
					d.Path = StripAngleBrackets(sp)
					havePath = true
					lastI = i
					break
				}
				// A TCP path may contain a port number already in it, or
				// consist of nothing but a port
				host, port, err := ParseHostPort(sp, "", UnknownPortNumber)
				if err != nil {
					return nil, parts, fmt.Errorf("Invalid TCP host/port in endpoint descriptor string'%s': '%s'", s, err)
				}
				if port == UnknownPortNumber {
					d.Path = host
					havePath = true
				} else {
					d.Path = host + ":" + port.String()
					havePath = true
					lastI = i
					break
				}

			}
		}
	}

	if d.Type == ChannelEndpointTypeUnknown {
		return nil, parts, fmt.Errorf("Unable to determine type from endpoint descriptor string '%s'", s)
	}

	if (d.Type == ChannelEndpointTypeUnix || d.Type == ChannelEndpointTypeLoop) && d.Path == "" {
		return nil, parts, fmt.Errorf("Missing endpoint path in endpoint descriptor string '%s'", s)
	}

	// We allow unspecified path for TCP because it is implicitly determined from remote
	// endpoint in some cases

	return d, parts[lastI+1:], nil
}

// ParseChannelEndpointDescriptor parses a single standalone channel endpoint descriptor string
func ParseChannelEndpointDescriptor(s string, role ChannelEndpointRole) (*ChannelEndpointDescriptor, error) {
	parts, err := SplitBracketedParts(s)
	if err != nil {
		return nil, fmt.Errorf("Badly formed channel endpoint descriptor '%s': %s", s, err)
	}
	d, remaining, err := ParseNextChannelEndpointDescriptor(parts, role)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 1 || (len(remaining) == 1 && remaining[0] != "") {
		return nil, fmt.Errorf("Too many parts in channel endpoint descriptor string: '%s'", s)
	}
	return d, nil
}

// ParseChannelDescriptor parses a string representing a ChannelDescriptor
func ParseChannelDescriptor(s string) (*ChannelDescriptor, error) {
	reverse := false
	parts, err := SplitBracketedParts(s)
	if err != nil {
		return nil, err
	}
	if len(parts) > 0 && parts[0] == "R" {
		reverse = true
		parts = parts[1:]
	}
	d := &ChannelDescriptor{
		Reverse: reverse,
	}

	var skeletonParts []string
	d.Stub, skeletonParts, err = ParseNextChannelEndpointDescriptor(parts, ChannelEndpointRoleStub)
	if err != nil {
		return nil, err
	}

	remParts := skeletonParts
	if len(skeletonParts) > 0 {
		d.Skeleton, remParts, err = ParseNextChannelEndpointDescriptor(parts, ChannelEndpointRoleSkeleton)
		if err != nil {
			return nil, err
		}
	} else {
		d.Skeleton = &ChannelEndpointDescriptor{Role: ChannelEndpointRoleSkeleton}
	}

	if len(remParts) != 0 {
		return nil, fmt.Errorf("Too many parts in channel descriptor string: '%s'", s)
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
	stubPort := UnknownPortNumber
	skeletonHost := ""
	skeletonPort := UnknownPortNumber

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if len(d.Stub.Path) > 0 {
			stubBindAddr, stubPort, err = ParseHostPort(d.Stub.Path, "", UnknownPortNumber)
			if err != nil {
				return nil, err
			}
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if len(d.Skeleton.Path) > 0 {
			skeletonHost, skeletonPort, err = ParseHostPort(d.Skeleton.Path, "", UnknownPortNumber)
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

	if d.Stub.Type == ChannelEndpointTypeTCP && stubPort == UnknownPortNumber {
		if d.Skeleton.Type == ChannelEndpointTypeSocks {
			stubPort = PortNumber(1080)
		} else if skeletonPort != UnknownPortNumber {
			stubPort = skeletonPort
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP && skeletonPort == UnknownPortNumber {
		if stubPort != UnknownPortNumber {
			skeletonPort = stubPort
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if stubBindAddr == "" {
			return nil, fmt.Errorf("Unable to determine stub bind address in channel descriptor string: '%s'", s)
		}
		if stubPort == UnknownPortNumber {
			return nil, fmt.Errorf("Unable to determine stub port number in channel descriptor string: '%s'", s)
		}

		d.Stub.Path = stubBindAddr + ":" + stubPort.String()
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if skeletonHost == "" {
			skeletonHost = "localhost"
		}
		if skeletonPort == UnknownPortNumber {
			return nil, fmt.Errorf("Unable to determine skeleton port number in channel descriptor string: '%s'", s)
		}
		d.Skeleton.Path = skeletonHost + ":" + skeletonPort.String()
	}

	if (d.Stub.Type == ChannelEndpointTypeStdio && d.Reverse) || (d.Skeleton.Type == ChannelEndpointTypeStdio && !d.Reverse) {
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

// ChannelEndpoint is a virtual network endpoint service of any type and role. Stub endpoints
// are able to listen for and accept connections from local network clients, and proxy
// them to a remote endpoint. Skeleton endpoints are able to accept connection requests from
// remote endpoints and proxy them to local network services.
type ChannelEndpoint interface {
	io.Closer
}

// AcceptorChannelEndpoint is a ChannelEndpoint that can be asked to accept and return connections from
// a Caller network client as expected by the endpoint configuration.
type AcceptorChannelEndpoint interface {
	ChannelEndpoint

	// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
	// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
	// this method if you need to begin accepting Callers before you make the first Accept call.
	StartListening() error

	// Accept listens for and accepts a single connection from a Caller network client as specified in the
	// endpoint configuration. This call does not return until a new connection is available or a
	// error occurs. There is no way to cancel an Accept() request other than closing the endpoint
	Accept(ctx context.Context) (ChannelConn, error)

	// AcceptAndServe listens for and accepts a single connection from a Caller network client as specified in the
	// endpoint configuration, then services the connection using an already established
	// calledServiceConn as the proxied Called Service's end of the session. This call does not return until
	// the bridged session completes or an error occurs. There is no way to cancel the Accept() portion
	// of the request other than closing the endpoint through other means. After the connection has been
	// accepted, the context may be used to cancel servicing of the active session.
	// Ownership of calledServiceConn is transferred to this function, and it will be closed before this function returns.
	// This API may be more efficient than separately using Accept() and then bridging between the two
	// ChannelConns with BasicBridgeChannels. In particular, "loop" endpoints can avoid creation
	// of a socketpair and an extra bridging goroutine, by directly coupling the acceptor ChannelConn
	// to the dialer ChannelConn.
	// The return value is a tuple consisting of:
	//        Number of bytes sent from the accepted callerConn to calledServiceConn
	//        Number of bytes sent from calledServiceConn to the accelpted callerConn
	//        An error, if one occured during accept or copy in either direction
	AcceptAndServe(ctx context.Context, calledServiceConn ChannelConn) (int64, int64, error)
}

// DialerChannelEndpoint is a ChannelEndpoint that can be asked to create a new connection to a network service
// as expected in the endpoint configuration.
type DialerChannelEndpoint interface {
	ChannelEndpoint

	// Dial initiates a new connection to a Called Service
	Dial(ctx context.Context, extraData []byte) (ChannelConn, error)

	// DialAndServe initiates a new connection to a Called Service as specified in the
	// endpoint configuration, then services the connection using an already established
	// callerConn as the proxied Caller's end of the session. This call does not return until
	// the bridged session completes or an error occurs. The context may be used to cancel
	// connection or servicing of the active session.
	// Ownership of callerConn is transferred to this function, and it will be closed before
	// this function returns, regardless of whether an error occurs.
	// This API may be more efficient than separately using Dial() and then bridging between the two
	// ChannelConns with BasicBridgeChannels. In particular, "loop" endpoints can avoid creation
	// of a socketpair and an extra bridging goroutine, by directly coupling the acceptor ChannelConn
	// to the dialer ChannelConn.
	// The return value is a tuple consisting of:
	//        Number of bytes sent from callerConn to the dialed calledServiceConn
	//        Number of bytes sent from the dialed calledServiceConn callerConn
	//        An error, if one occured during dial or copy in either direction
	DialAndServe(
		ctx context.Context,
		callerConn ChannelConn,
		extraData []byte,
	) (int64, int64, error)
}

// LocalStubChannelEndpoint is an AcceptorChannelEndpoint that accepts connections from local network clients
type LocalStubChannelEndpoint interface {
	AcceptorChannelEndpoint
}

// LocalSkeletonChannelEndpoint is a Dialer
type LocalSkeletonChannelEndpoint interface {
	DialerChannelEndpoint
}

// ChannelConn is a virtual open "socket", either
//      1) created by a ChannelEndpoint to wrap communication with a local network resource
//      2) created by the proxy session to wrap a single ssh communication channel with a remote endpoint
type ChannelConn interface {
	io.ReadWriteCloser

	// WaitForClose blocks until the Close() method has been called and completed. The error returned
	// from the first Close() is returned
	WaitForClose() error

	// CloseWrite shuts down the writing side of the "socket". Corresponds to net.TCPConn.CloseWrite().
	// this method is called when end-of-stream is reached reading from the other ChannelConn of a pair
	// pair are connected via a ChannelPipe. It allows for protocols like HTTP 1.0 in which a client
	// sends a request, closes the write side of the socket, then reads the response, and a server reads
	// a request until end-of-stream before sending a response.
	CloseWrite() error

	// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
	GetNumBytesRead() int64

	// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
	GetNumBytesWritten() int64
}

// BasicEndpoint is a base common implementation for local ChannelEndPoints
type BasicEndpoint struct {
	*Logger
	lock   sync.Mutex
	ced    *ChannelEndpointDescriptor
	closed bool
}

// BasicConn is a base common implementation for local ChannelConn
type BasicConn struct {
	ChannelConn
	*Logger
	Lock            sync.Mutex
	Done            chan struct{}
	CloseOnce       sync.Once
	CloseErr        error
	NumBytesRead    int64
	NumBytesWritten int64
}

// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
func (c *BasicConn) GetNumBytesRead() int64 {
	return atomic.LoadInt64(&c.NumBytesRead)
}

// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
func (c *BasicConn) GetNumBytesWritten() int64 {
	return atomic.LoadInt64(&c.NumBytesWritten)
}

// BasicBridgeChannels connects two ChannelConn's together, copying betweeen them bi-directionally
// until end-of-stream is reached in both directions. Both channels are closed before this function
// returns. Three values are returned:
//    Number of bytes transferred from caller to calledService
//    Number of bytes transferred from calledService to caller
//    If io.Copy() returned an error in either direction, the error value.
//
// CloseWrite() is called on each channel after transfer to that channel is complete.
//
// Currently the context is not used and there is no way to cancel the bridge without closing
// one of the ChannelConn's.
func BasicBridgeChannels(
	ctx context.Context,
	caller ChannelConn,
	calledService ChannelConn,
) (int64, int64, error) {
	var callerToServiceBytes, serviceToCallerBytes int64
	var callerToServiceErr, serviceToCallerErr error
	var wg sync.WaitGroup
	wg.Add(2)
	copyFunc := func(src ChannelConn, dst ChannelConn, bytesCopied *int64, copyErr *error) {
		// Copy from caller to calledService
		*bytesCopied, *copyErr = io.Copy(dst, src)
		dst.CloseWrite()
		wg.Done()
	}
	go copyFunc(caller, calledService, &callerToServiceBytes, &callerToServiceErr)
	go copyFunc(calledService, caller, &serviceToCallerBytes, &serviceToCallerErr)
	wg.Wait()
	calledService.Close()
	caller.Close()
	err := callerToServiceErr
	if err == nil {
		err = serviceToCallerErr
	}
	return callerToServiceBytes, serviceToCallerBytes, err
}

// LocalChannelEnv provides necessary context for initialization of local channel endpoints
type LocalChannelEnv interface {
	// IsServer returns true if this is a proxy server; false if it is a cliet
	IsServer() bool

	// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
	GetLoopServer() *LoopServer

	// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
	// nil otherwise
	GetSocksServer() *socks5.Server
}

// NewLocalStubChannelEndpoint creates a LocalStubChannelEndpoint from its descriptor
func NewLocalStubChannelEndpoint(
	logger *Logger,
	env LocalChannelEnv,
	ced *ChannelEndpointDescriptor,
) (LocalStubChannelEndpoint, error) {
	var ep LocalStubChannelEndpoint
	var err error

	if ced.Role != ChannelEndpointRoleStub {
		err = fmt.Errorf("%s: Role must be stub: %s", logger.Prefix(), ced.LongString())
	} else if ced.Type == ChannelEndpointTypeStdio {
		if env.IsServer() {
			err = fmt.Errorf("%s: stdio endpoints are not allowed on the server side: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewStdioStubEndpoint(logger, ced)
		}
	} else if ced.Type == ChannelEndpointTypeLoop {
		loopServer := env.GetLoopServer()
		if loopServer == nil {
			err = fmt.Errorf("%s: Loop endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewLoopStubEndpoint(logger, ced, loopServer)
		}
	} else if ced.Type == ChannelEndpointTypeTCP {
		ep, err = NewTCPStubEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeUnix {
		ep, err = NewUnixStubEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeSocks {
		err = fmt.Errorf("%s: Socks endpoint Role must be skeleton: %s", logger.Prefix(), ced.LongString())
	} else {
		err = fmt.Errorf("%s: Unsupported endpoint type '%s': %s", logger.Prefix(), ced.Type, ced.LongString())
	}

	return ep, err
}

// NewLocalSkeletonChannelEndpoint creates a LocalSkeletonChannelEndpoint from its descriptor
func NewLocalSkeletonChannelEndpoint(
	logger *Logger,
	env LocalChannelEnv,
	ced *ChannelEndpointDescriptor,
) (LocalSkeletonChannelEndpoint, error) {
	var ep LocalSkeletonChannelEndpoint
	var err error

	if ced.Role != ChannelEndpointRoleSkeleton {
		err = fmt.Errorf("%s: Role must be skeleton: %s", logger.Prefix(), ced.LongString())
	} else if ced.Type == ChannelEndpointTypeStdio {
		if env.IsServer() {
			err = fmt.Errorf("%s: stdio endpoints are not allowed on the server side: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewStdioSkeletonEndpoint(logger, ced)
		}
	} else if ced.Type == ChannelEndpointTypeLoop {
		loopServer := env.GetLoopServer()
		if loopServer == nil {
			err = fmt.Errorf("%s: Loop endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewLoopSkeletonEndpoint(logger, ced, loopServer)
		}
	} else if ced.Type == ChannelEndpointTypeTCP {
		ep, err = NewTCPSkeletonEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeUnix {
		ep, err = NewUnixSkeletonEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeSocks {
		socksServer := env.GetSocksServer()
		if socksServer == nil {
			err = fmt.Errorf("%s: socks endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewSocksSkeletonEndpoint(logger, ced, socksServer)
		}
	} else {
		err = fmt.Errorf("%s: Unsupported endpoint type '%s': %s", logger.Prefix(), ced.Type, ced.LongString())
	}

	return ep, err
}
