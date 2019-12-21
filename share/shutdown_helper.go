package chshare

import "context"
import "sync"

// OnceShutdownHandler is an interface that must be implemented by the object managed by ShutdownHelper
type OnceShutdownHandler interface {
	// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
	// as an advisory completion value, actually shut down, then return the real completion value.
	HandleOnceShutdown(completionError error) error
}

/*
// ShutdownDoneChanProvider is an interface implemented by objects that provide
// a chan that is closed when they are completely shut down.
type ShutdownDoneChanProvider interface {
	ShutdownDoneChan() <-chan struct{}
}
*/

// AsyncShutdowner is an interface implemented by objects that provide
// asynchronous shutdown capability.
type AsyncShutdowner interface {
	// StartShutdown asynchronously begins shutting down the object. If the object
	// has already begun shutting down, it has no effect.
	// completionErr is an advisory error (or nil) to use as the completion status
	// from WaitShutdown(). The implementation may use this value or decide to return
	// something else.
	StartShutdown(completionErr error)

	// ShutdownDoneChan returns a chan that is closed after shutdown is complete.
	// After this channel is closed, it is guaranteed that IsDoneShutdown() will
	// return true, and WaitForShutdown will not block.
	ShutdownDoneChan() <-chan struct{}

	// IsDoneShutdown returns false if the object is not yet completely
	// shut down. Otherwise it returns true with the guarantee that
	// ShutDownDoneChan() will be immediately closed and WaitForShutdown
	// will immediately return the final status.
	IsDoneShutdown() bool

	// WaitShutdown blocks until the object is completely shut down, and
	// returns the final completion status
	WaitShutdown() error
}

// ShutdownHelper is a base that manages clean asynchronous object shutdown for an
// object that implements OnceShutdownHandler
type ShutdownHelper struct {
	// Logger is the Logger that will be used for log output from this helper
	*Logger

	// Lock is a general-purpose fine-grained mutex for this helper; it may be used
	// as a general-purpose lock by derived objects as well
	Lock sync.Mutex

	// The object that is being managed by this helper, which is called exactly once
	// to perform synchronous shutdown.
	shutdownHandler OnceShutdownHandler

	// isStartedShutdown is set to true when we begin shutting down
	isStartedShutdown bool

	// isDoneShutdown is set to true when shutdown is completely done
	isDoneShutdown bool

	// shutdownErr contains the final completion statis after isDoneShutdown is true
	shutdownErr error

	// shutdownStartedChan is a chan that is closed when shutdown is started
	shutdownStartedChan chan struct{}

	// shutdownDoneChan is a chan that is closed when shutdown is completely done
	shutdownDoneChan chan struct{}

	// children is a list of child objects who will be actively shut down by this helper
	// after shutdowHandler has been called, before shutdown is considered completely done.
	children []AsyncShutdowner

	// childDoneChans is a list of arbitrary chans that someone external to this helper
	// will close before this helper will consider shutdown to be complete.
	childDoneChans []<-chan struct{}

	// wg is a sync.WaitGroup that this helper will wait on before it considers shutdown
	// to be complete.
	wg sync.WaitGroup
}

// InitShutdownHelper initializes a new ShutdownHelper in place
func (h *ShutdownHelper) InitShutdownHelper(
	logger *Logger,
	shutdownHandler OnceShutdownHandler,
) {
	h.Logger = logger
	h.shutdownHandler = shutdownHandler
	h.shutdownStartedChan = make(chan struct{})
	h.shutdownDoneChan = make(chan struct{})
}

// NewShutdownHelper creates a new ShutdownHelper on the heap
func NewShutdownHelper(
	logger *Logger,
	shutdownHandler OnceShutdownHandler,
) *ShutdownHelper {
	h := &ShutdownHelper{
		Logger:              logger,
		shutdownHandler:     shutdownHandler,
		shutdownStartedChan: make(chan struct{}),
		shutdownDoneChan:    make(chan struct{}),
	}
	return h
}

// ShutdownOnContext begins background monitoring of a context.Context, and
// will begin asynchronously shutting down this helper with the context's error
// if the context is completed. This method does not block, it just
// constrains the lifetime of this object to a context.
func (h *ShutdownHelper) ShutdownOnContext(ctx context.Context) {
	go func() {
		select {
		case <-h.shutdownStartedChan:
		case <-ctx.Done():
			h.StartShutdown(ctx.Err())
		}
	}()
}

// IsStartedShutdown returns true if shutdown has begun. It continues to return true after shutdown
// is complete
func (h *ShutdownHelper) IsStartedShutdown() bool {
	h.Lock.Lock()
	defer h.Lock.Unlock()
	return h.isStartedShutdown
}

// IsDoneShutdown returns true if shutdown is complete.
func (h *ShutdownHelper) IsDoneShutdown() bool {
	h.Lock.Lock()
	defer h.Lock.Unlock()
	return h.isDoneShutdown
}

// ShutdownWG returns a sync.WaitGroup that you can call Add() on to
// defer final completion of shutdown until the specified number of calls to
// ShutdownWG().Done() are made
func (h *ShutdownHelper) ShutdownWG() *sync.WaitGroup {
	return &h.wg
}

// ShutdownStartedChan returns a channel that will be closed as soon as shutdown is initiated
func (h *ShutdownHelper) ShutdownStartedChan() <-chan struct{} {
	return h.shutdownDoneChan
}

// ShutdownDoneChan returns a channel that will be closed after shutdown is done
func (h *ShutdownHelper) ShutdownDoneChan() <-chan struct{} {
	return h.shutdownDoneChan
}

// WaitShutdown waits for the shutdown to complete, then returns the shutdown status
// It does not initiate shutdown, so it can be used to wait on an object that
// will shutdown at an unspecified point in the future.
func (h *ShutdownHelper) WaitShutdown() error {
	<-h.shutdownDoneChan
	return h.shutdownErr
}

// Shutdown performs a synchronous shutdown, It initiates shutdown if it has
// not already started, waits for the shutdown to comlete, then returns
// the final shutdown status
func (h *ShutdownHelper) Shutdown(completionError error) error {
	h.StartShutdown(completionError)
	return h.WaitShutdown()
}

// StartShutdown asynchronously begins shutting down the object. If the object
// has already begun shutting down, it has no effect.
// completionError is an advisory error (or nil) to use as the completion status
// from WaitShutdown(). The implementation may use this value or decide to return
// something else.
//
// Asynchronously, the help kick off the following, only the first time it is called:
//
//  -   signals that shutdown has startedd
//  -   invokes HandleOnceShutdown with the provided avdvisory completion status. The
//       return value will be used as the final completion status for shutdown
//  -   For each registered child, calls StartShutdown, using the return value from
//       HandleOnceShutdown as an advirory completion status.
//  -   For each registered child, waits for the
//       child to finish shuting down
//  -   For each manually added child done chan, waits for the
//       child done chan to be closed
//  -   Waits for the wait group count to reach 0
//  -   Signals shutdown complete, using the return value from HandleOnceShutdown
//  -    as the final completion code
func (h *ShutdownHelper) StartShutdown(completionErr error) {
	h.Lock.Lock()
	isStartedShutdown := h.isStartedShutdown
	h.isStartedShutdown = true
	h.Lock.Unlock()

	if !isStartedShutdown {
		close(h.shutdownStartedChan)
		go func() {
			h.shutdownErr = h.shutdownHandler.HandleOnceShutdown(completionErr)
			h.Lock.Lock()
			children := h.children
			childDoneChans := h.childDoneChans
			h.Lock.Unlock()
			for _, child := range children {
				child.StartShutdown(h.shutdownErr)
			}
			for _, child := range children {
				<-child.ShutdownDoneChan()
			}
			for _, childDoneChan := range childDoneChans {
				<-childDoneChan
			}
			h.wg.Wait()

			h.isDoneShutdown = true
			close(h.shutdownDoneChan)
		}()
	}
}

// Close is a default implementation of Close(), which simply shuts down
// with an advisory completion status of nil, and returns the final completion
// status
func (h *ShutdownHelper) Close() error {
	return h.Shutdown(nil)
}

// AddShutdownChildChan adds a chan that will be waited on before this object's shutdown is
// considered complete. The helper will not take any action to cause the chan to be
// closed; it is the caller's responsibility to do that.
func (h *ShutdownHelper) AddShutdownChildChan(childDoneChan <-chan struct{}) {
	h.Lock.Lock()
	h.childDoneChans = append(h.childDoneChans, childDoneChan)
	h.Lock.Unlock()
}

// AddShutdownChild adds a child object to the set of objects that will be
// actively shut down by this helper after HandleOnceShutdown() returns, before this
// object's shutdown is considered complete.
func (h *ShutdownHelper) AddShutdownChild(child AsyncShutdowner) {
	h.Lock.Lock()
	h.children = append(h.children, child)
	h.Lock.Unlock()
}
