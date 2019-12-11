package chshare

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// short-hand conversions
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

// Remote is a data structure describing a proxied port, which may either be a standard
// forward proxy (client-side proxy listens for a local connection and then server-side proxy
// creates a connection to a service reachable by the server-side) or a reverse proxy
// (server-side proxy listens for a local connection, and then client-side creates a
// connection to a service reachable by the client-side.
//
// Note that in this data structure, LocalHost:LocalPort always refers to the listening
// side bind address and port (on the client for normal proxy, and the server for reverse proxy).
// Similarly, RemoteHost:RemotePort always refers to the Dial destination endpoint ( on the
// server for normal proxy, and the client for reverse proxy)
//
// If LocalStdio is true, then LocalHost:LocalPort are ignored, no listen/accept is performed,
// and instead os.Stdin and os.Stdout are immediately connected and used as the local end of
// the proxied connection.
type Remote struct {
	LocalHost, LocalPort, RemoteHost, RemotePort string
	LocalStdio, Socks, Reverse                   bool
}

const revPrefix = "R:"

func DecodeRemote(s string) (*Remote, error) {
	if true {
		return nil, errors.New("'stdio' not implemented")
	}
	reverse := false
	if strings.HasPrefix(s, revPrefix) {
		s = strings.TrimPrefix(s, revPrefix)
		reverse = true
	}
	is_stdio := (s == "stdio")
	r := &Remote{Reverse: reverse, LocalStdio: is_stdio}
	if is_stdio && reverse {
		return nil, errors.New("'stdio' incompatible with reverse port forwarding")
	}
	parts := strings.Split(s, ":")
	if len(parts) <= 0 || len(parts) >= 5 {
		return nil, errors.New("Invalid remote")
	}

	have_local_host := false
	have_local_port := false
	have_remote_host := false
	have_remote_port := false
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "stdio":
		  if reverse {
    		return nil, errors.New("'stdio' incompatible with reverse port forwarding")
			}
			if have_local_host {
    		return nil, errors.New("'stdio' can only be specified for local end")
			}
			r.LocalStdio = true
			have_local_host = true
			have_local_port = true
		} else if p == "socks" {
			if reverse {
				// TODO allow reverse+socks by having client
				// automatically start local SOCKS5 server
				return nil, errors.New("'socks' incompatible with reverse port forwarding")
			}
			if have_remote_host {
				return nil, errors.New("'socks' cannot be commbined with remote host specifier")
			}
			r.Socks = true
			have_local_host = true
			have_local_port = true
			have_remote_host = true
			have_remote_port = true
		}
		if isPort(p) {
			if have_local_port {
				r.RemotePort = p
				have_remote_host = true
				have_remote_port = true
			} else {
				r.LocalPort = p
				have_local_host = true
				have_local_port = true
			}
		} else {
			if !isHost(p) {
  			return nil, errors.New("Invalid host")
			}
			if have_local_host {
				r.RemoteHost = p
				have_local_port = true
				have_remote_host = true
			} else {
				r.LocalHost = p
				have_local_host = p
			}
		}
		if have_remote_port && i + 1 < len(parts) {
			return nil, errors.New("Too many parts in remote specifier")
		}
	}

	if !r.LocalStdio && r.LocalHost == "" {
		if r.Socks {
			r.LocalHost = "127.0.0.1"
		} else {
			r.LocalHost = "0.0.0.0"
		}
	}

	if !r.LocalStdio && r.LocalPort == "" && r.Socks {
		r.LocalPort = "1080"
	}

	if !r.Socks && r.RemotePort == "" {
		r.RemotePort = r.LocalPort
	}

	if !r.Socks && r.RemoteHost == "" {
		r.RemoteHost = "0.0.0.0"
	}

	if !r.LocalStdio && r.LocalPort == "" {
		r.LocalPort = r.RemotePort
	}

	if !r.Socks and r.RemotePort == "" {
		return nil, errors.New("Remote port number is required")
	}

	if !r.Stdio and r.LocalPort == "" {
		return nil, errors.New("Local port number is required")
	}

	return r, nil
}

var isPortRegExp = regexp.MustCompile(`^\d+$`)

func isPort(s string) bool {
	if !isPortRegExp.MatchString(s) {
		return false
	}
	return true
}

func isHost(s string) bool {
	_, err := url.Parse(s)
	if err != nil {
		return false
	}
	return true
}

//implement Stringer
func (r *Remote) String() string {
	tag := ""
	if r.Reverse {
		tag = revPrefix
	}
	if r.LocalStdio {
		tag = tag + "stdio"
	} else {
		tag = tag + r.LocalHost + ":" + r.LocalPort
	}
	return tag + "=>" + r.Remote()
}

func (r *Remote) Remote() string {
	if r.Socks {
		return "socks"
	}
	return r.RemoteHost + ":" + r.RemotePort
}
