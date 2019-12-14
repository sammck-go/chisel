package chshare
/*

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const revPrefix = "R:"



func DecodeRemote(s string) (*Remote, error) {
	reverse := false
	if strings.HasPrefix(s, revPrefix) {
		s = strings.TrimPrefix(s, revPrefix)
		reverse = true
	}
	isStdio := (s == "stdio")
	r := &Remote{Reverse: reverse, LocalStdio: isStdio}
	if isStdio && reverse {
		return nil, errors.New("'stdio' incompatible with reverse port forwarding")
	}
	parts := strings.Split(s, ":")
	if len(parts) <= 0 || len(parts) >= 5 {
		return nil, errors.New("Invalid remote")
	}

	haveLocalHost := false
	haveLocalPort := false
	haveRemoteHost := false
	haveRemotePort := false
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "stdio" {
			if reverse {
				return nil, errors.New("'stdio' incompatible with reverse port forwarding")
			}
			if haveLocalHost {
				return nil, errors.New("'stdio' can only be specified for local end")
			}
			r.LocalStdio = true
			haveLocalHost = true
			haveLocalPort = true
		} else if p == "socks" {
			if reverse {
				// TODO allow reverse+socks by having client
				// automatically start local SOCKS5 server
				return nil, errors.New("'socks' incompatible with reverse port forwarding")
			}
			if haveRemoteHost {
				return nil, errors.New("'socks' cannot be commbined with remote host specifier")
			}
			r.Socks = true
			haveLocalHost = true
			haveLocalPort = true
			haveRemoteHost = true
			haveRemotePort = true
		} else if isPort(p) {
			if haveLocalPort {
				r.RemotePort = p
				haveRemoteHost = true
				haveRemotePort = true
			} else {
				r.LocalPort = p
				haveLocalHost = true
				haveLocalPort = true
			}
		} else {
			if !isHost(p) {
				return nil, errors.New("Invalid host")
			}
			if haveLocalHost {
				r.RemoteHost = p
				haveLocalPort = true
				haveRemoteHost = true
			} else {
				r.LocalHost = p
				haveLocalHost = true
			}
		}
		if haveRemotePort && i+1 < len(parts) {
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

	if !r.Socks && r.RemotePort == "" {
		return nil, errors.New("Remote port number is required")
	}

	if !r.LocalStdio && r.LocalPort == "" {
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
*/
