package chshare

import (
	"fmt"
	"sync"
)

// LoopServer implements a namespace for "loop" endpoints to connect

type loopEntry struct {
	name string
	lock sync.Mutex
	acceptor *LoopStubEndpoint
	dialers chan *LoopSkeletonEndpoint
}

// LoopServer coordinates connection between a LoopStubEndpoint and any number
// of LoopSkeletonEndpoint's bound to the same name. 
type LoopServer struct {
	*Logger
	lock   sync.Mutex
	entries map[string]*loopEntry
}

// NewLoopServer creates a new LoopServer
func NewLoopServer(logger *Logger) (*LoopServer, error) {
	s := &LoopServer{
		Logger: logger.Fork("LoopServer"),
	}
	return s, nil
}

func (s *LoopServer) String() string {
	return s.Logger.Prefix()
}

func(s *LoopServer) getEntry(name string, create bool) *loopEntry {
	s.lock.Lock()
	defer s.lock.Unlock()
	entry, ok := s.entries[name]
	if !ok {
		if create {
			entry = &loopEntry{name: name}
			s.entries[name] = entry
		} else {
			entry = nil
		}
	}
	return entry
}

// RegisterAcceptor registers a LoopStubEndpoint as the acceptor for a given loop name. 
// Only one acceptor can be registered at a given time with a given name
func (s *LoopServer) RegisterAcceptor(name string, acceptor *LoopStubEndpoint, create bool) error {
	entry := s.getEntry(name, create)
	if entry == nil {
		return fmt.Errorf("%s: Loopback name does not exist: %s", s.Logger.Prefix(), name)
	}
	entry.lock.Lock()
	defer entry.lock.Unlock()
	if entry.acceptor != nil {
		return fmt.Errorf("%s: Loopback acceptor already registered for name: %s", s.Logger.Prefix(), name)
	}
	entry.acceptor = acceptor
	// TODO start goroutine servicing acceptor
	return nil
}

// RegisterDialerConn registers a LoopSkeletonEndpoint as a dialer for a given loop name. 
// Only one acceptor can be registered at a given time with a given name
func (s *LoopServer) RegisterAcceptor(name string, acceptor *LoopStubEndpoint, create bool) error {
	entry := s.getEntry(name, create)
	if entry == nil {
		return fmt.Errorf("%s: Loopback name does not exist: %s", s.Logger.Prefix(), name)
	}
	entry.lock.Lock()
	defer entry.lock.Unlock()
	if entry.acceptor != nil {
		return fmt.Errorf("%s: Loopback acceptor already registered for name: %s", s.Logger.Prefix(), name)
	}
	entry.acceptor = acceptor
	// TODO start goroutine servicing acceptor
	return nil
}


