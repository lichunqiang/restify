package core

import (
	"sync"
	"time"
)

// LockHooks report the rest session's course. They run on the session's
// goroutine.
type LockHooks struct {
	Tick func(remaining time.Duration) // the countdown moved
	Done func()                        // the rest ran out: unlock quietly
}

// LockSession is the forced rest: a countdown during which Restify covers
// the screens, ending by itself or when the user ends it. The Swift app
// called this the "lock" session; it covers the screens but locks nothing.
type LockSession struct {
	hooks LockHooks

	// TickEvery is the countdown's step; tests shrink it.
	TickEvery time.Duration

	mu    sync.Mutex
	end   time.Time
	total time.Duration
	stop  chan struct{}
}

// NewLockSession makes a session with a one-second countdown.
func NewLockSession(hooks LockHooks) *LockSession {
	return &LockSession{hooks: hooks, TickEvery: time.Second}
}

// Start begins a rest of duration, ending by itself when it runs out.
func (l *LockSession) Start(duration time.Duration, now time.Time) {
	l.mu.Lock()
	if l.stop != nil {
		close(l.stop)
	}
	l.end = now.Add(duration)
	l.total = duration
	l.stop = make(chan struct{})
	stop := l.stop
	remaining := duration
	l.mu.Unlock()

	l.fireTick(remaining)
	go l.loop(stop)
}

func (l *LockSession) loop(stop chan struct{}) {
	ticker := time.NewTicker(l.TickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			l.mu.Lock()
			if l.stop == nil {
				l.mu.Unlock()
				return
			}
			remaining := time.Until(l.end)
			expired := remaining <= 0
			if expired {
				l.stop = nil
				l.end = time.Time{}
				l.total = 0
			}
			l.mu.Unlock()

			if expired {
				l.fire0(l.hooks.Done)
				return
			}
			l.fireTick(remaining)
		}
	}
}

// Stop ends the rest now (the user pressed "force exit" or "done").
func (l *LockSession) Stop() {
	l.mu.Lock()
	stop := l.stop
	l.stop = nil
	l.end = time.Time{}
	l.total = 0
	l.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// Active reports whether a rest is running.
func (l *LockSession) Active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stop != nil
}

// Remaining reports how long the rest has left (zero when none).
func (l *LockSession) Remaining() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop == nil {
		return 0
	}
	r := time.Until(l.end)
	if r < 0 {
		r = 0
	}
	return r
}

// fire runs a hook that may be nil.
func (l *LockSession) fire0(fn func()) {
	if fn != nil {
		fn()
	}
}

func (l *LockSession) fireTick(remaining time.Duration) {
	if l.hooks.Tick != nil {
		l.hooks.Tick(remaining)
	}
}
