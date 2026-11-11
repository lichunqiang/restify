package core

import (
	"sync"
	"time"
)

// Phase is the work timer's state.
type Phase int

const (
	PhaseIdle Phase = iota
	PhaseRunning
	PhasePaused
)

func (p Phase) String() string {
	switch p {
	case PhaseRunning:
		return "运行中"
	case PhasePaused:
		return "已暂停"
	default:
		return "未开始"
	}
}

// Snapshot is one look at the work timer's state.
type Snapshot struct {
	Phase     Phase
	Remaining time.Duration
	Total     time.Duration
}

// WorkDeps injects what the timer needs from the system.
type WorkDeps struct {
	// IdleTime reports how long since the last user input. Nil disables
	// the idle auto-pause and the smart reminder.
	IdleTime func() time.Duration
	// Now is the clock; nil uses time.Now.
	Now func() time.Time
}

// WorkHooks report what the timer does. They run on the timer's
// goroutines, or on the goroutine that called a control.
type WorkHooks struct {
	Tick          func(Snapshot)                                  // every second while running
	Change        func(Snapshot)                                  // the phase changed
	Break         func(Snapshot)                                  // the work period ended: rest, or notify
	Persist       func(s Snapshot, start time.Time, running bool) // persist request
	IdlePause     func()                                          // auto-paused: the system was idle 5 minutes
	SmartReminder func()                                          // the user worked threshold-long without the timer
	Blink         func(bool)                                      // menu-bar icon blink on/off
}

// WorkTimer counts one work period down, pausing when the system goes
// idle and nudging the user to start it when they work without it.
type WorkTimer struct {
	deps  WorkDeps
	hooks WorkHooks

	// Tunables; the defaults are the Swift app's, and tests shrink them.
	TickEvery     time.Duration // 1s
	PersistEvery  time.Duration // 1min
	IdlePoll      time.Duration // 1min
	IdleThreshold time.Duration // 5min
	SmartPoll     time.Duration // 30s
	ActiveBelow   time.Duration // idle < 1min counts as active

	mu        sync.Mutex
	phase     Phase
	remaining time.Duration
	total     time.Duration
	startedAt time.Time // when this run began (wall clock, for persistence)

	smartEnabled   bool
	smartThreshold time.Duration

	stopTick    chan struct{}
	stopIdle    chan struct{}
	stopSmart   chan struct{}
	activeSince time.Time
}

// NewWorkTimer makes a timer with the default tunables.
func NewWorkTimer(deps WorkDeps, hooks WorkHooks) *WorkTimer {
	return &WorkTimer{
		deps:           deps,
		hooks:          hooks,
		TickEvery:      time.Second,
		PersistEvery:   time.Minute,
		IdlePoll:       time.Minute,
		IdleThreshold:  5 * time.Minute,
		SmartPoll:      30 * time.Second,
		ActiveBelow:    time.Minute,
		smartThreshold: 5 * time.Minute,
	}
}

// SetSmartReminder configures the smart nudge. It starts or stops the
// monitor to match.
func (t *WorkTimer) SetSmartReminder(enabled bool, threshold time.Duration) {
	t.mu.Lock()
	t.smartEnabled = enabled
	if threshold > 0 {
		t.smartThreshold = threshold
	}
	start := enabled && t.phase == PhaseIdle && t.deps.IdleTime != nil
	t.mu.Unlock()
	if start {
		t.startSmart()
	} else {
		t.stopSmartMonitor()
	}
}

func (t *WorkTimer) now() time.Time {
	if t.deps.Now != nil {
		return t.deps.Now()
	}
	return time.Now()
}

// fire runs a hook that may be nil.
func (t *WorkTimer) fire(fn func(bool), on bool) {
	if fn != nil {
		fn(on)
	}
}

func (t *WorkTimer) fireS(fn func(Snapshot), s Snapshot) {
	if fn != nil {
		fn(s)
	}
}

func (t *WorkTimer) fire0(fn func()) {
	if fn != nil {
		fn()
	}
}

func (t *WorkTimer) firePersist(s Snapshot, start time.Time, running bool) {
	if t.hooks.Persist != nil {
		t.hooks.Persist(s, start, running)
	}
}

// Snapshot reads the current state.
func (t *WorkTimer) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Snapshot{Phase: t.phase, Remaining: t.remaining, Total: t.total}
}

// Start begins a work period of duration, as a fresh run.
func (t *WorkTimer) Start(duration time.Duration) {
	t.mu.Lock()
	if t.phase == PhaseRunning {
		t.mu.Unlock()
		return
	}
	t.phase = PhaseRunning
	t.total = duration
	t.remaining = duration
	t.startedAt = t.now()
	started := t.startedAt
	snap := t.snapshotLocked()
	t.launchTickLocked()
	t.mu.Unlock()

	t.stopSmartMonitor()
	t.fire(t.hooks.Blink, false)
	t.firePersist(snap, started, true)
	t.fireS(t.hooks.Change, snap)
}

// Pause stops the countdown, keeping what remains.
func (t *WorkTimer) Pause() {
	t.mu.Lock()
	if t.phase != PhaseRunning {
		t.mu.Unlock()
		return
	}
	t.phase = PhasePaused
	snap := t.snapshotLocked()
	started := t.startedAt
	stopTick, stopIdle := t.stopTick, t.stopIdle
	t.stopTick, t.stopIdle = nil, nil
	t.mu.Unlock()

	if stopTick != nil {
		close(stopTick)
	}
	if stopIdle != nil {
		close(stopIdle)
	}
	t.stopSmartMonitor() // a paused timer does not nudge
	t.fire(t.hooks.Blink, false)
	t.firePersist(snap, started, false)
	t.fireS(t.hooks.Change, snap)
}

// Resume continues a paused countdown where it left off.
func (t *WorkTimer) Resume() {
	t.mu.Lock()
	if t.phase != PhasePaused {
		t.mu.Unlock()
		return
	}
	t.phase = PhaseRunning
	// Wind the persisted start time to what the remaining implies.
	t.startedAt = t.now().Add(-t.total + t.remaining)
	started := t.startedAt
	snap := t.snapshotLocked()
	t.launchTickLocked()
	t.mu.Unlock()

	t.stopSmartMonitor()
	t.fire(t.hooks.Blink, false)
	t.firePersist(snap, started, true)
	t.fireS(t.hooks.Change, snap)
}

// Reset stops everything and goes idle, ending any rest session first (the
// caller's Reset hook unlocks the overlay).
func (t *WorkTimer) Reset() {
	t.mu.Lock()
	t.phase = PhaseIdle
	t.remaining, t.total, t.startedAt = 0, 0, time.Time{}
	snap := t.snapshotLocked()
	stopTick, stopIdle := t.stopTick, t.stopIdle
	t.stopTick, t.stopIdle = nil, nil
	t.mu.Unlock()

	if stopTick != nil {
		close(stopTick)
	}
	if stopIdle != nil {
		close(stopIdle)
	}
	t.firePersist(snap, time.Time{}, false)
	t.fireS(t.hooks.Change, snap)
	if t.smartWanted() {
		t.startSmart()
	}
}

// SkipBreak ends the rest and starts the next work period.
func (t *WorkTimer) SkipBreak(duration time.Duration) {
	t.Reset()
	t.Start(duration)
}

// Toggle starts a paused or idle timer, pauses a running one.
func (t *WorkTimer) Toggle(duration time.Duration) {
	switch t.Snapshot().Phase {
	case PhaseRunning:
		t.Pause()
	default:
		t.Start(duration)
	}
}

// snapshotLocked reads the state; the caller holds mu.
func (t *WorkTimer) snapshotLocked() Snapshot {
	return Snapshot{Phase: t.phase, Remaining: t.remaining, Total: t.total}
}

// launchTickLocked starts the countdown and the idle monitor; the caller
// holds mu. No countdown loop can be alive here: only Start and Resume
// call it, both of which leave a phase the loops had already exited.
func (t *WorkTimer) launchTickLocked() {
	t.stopTick = make(chan struct{})
	go t.tickLoop(t.stopTick)
	if t.deps.IdleTime != nil {
		t.stopIdle = make(chan struct{})
		go t.idleLoop(t.stopIdle)
	}
}

func (t *WorkTimer) tickLoop(stop chan struct{}) {
	ticker := time.NewTicker(t.TickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !t.tick() {
				return
			}
		}
	}
}

// tick decrements once; it reports false when the countdown ended and the
// loop should stop.
func (t *WorkTimer) tick() bool {
	t.mu.Lock()
	if t.phase != PhaseRunning {
		stop := t.stopTick
		t.stopTick = nil
		t.mu.Unlock()
		if stop != nil {
			close(stop)
		}
		return false
	}
	t.remaining -= t.TickEvery
	if t.remaining <= 0 {
		t.remaining = 0
		t.phase = PhaseIdle
		snap := t.snapshotLocked()
		stopTick, stopIdle := t.stopTick, t.stopIdle
		t.stopTick, t.stopIdle = nil, nil
		t.mu.Unlock()

		if stopTick != nil {
			close(stopTick)
		}
		if stopIdle != nil {
			close(stopIdle)
		}
		t.firePersist(snap, time.Time{}, false)
		t.fireS(t.hooks.Break, snap)
		t.fireS(t.hooks.Change, snap)
		// Nudge monitoring starts a beat later, so the rest's own events
		// don't count as activity.
		time.AfterFunc(5*t.TickEvery, func() {
			if t.smartWanted() {
				t.startSmart()
			}
		})
		return false
	}
	snap := t.snapshotLocked()
	persist := snap.Remaining%t.PersistEvery == 0
	started := t.startedAt
	t.mu.Unlock()

	t.fireS(t.hooks.Tick, snap)
	if persist {
		t.firePersist(snap, started, true)
	}
	return true
}

func (t *WorkTimer) idleLoop(stop chan struct{}) {
	ticker := time.NewTicker(t.IdlePoll)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if t.deps.IdleTime() > t.IdleThreshold && t.Snapshot().Phase == PhaseRunning {
				t.Pause()
				t.fire0(t.hooks.IdlePause)
			}
		}
	}
}

func (t *WorkTimer) smartWanted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.smartEnabled && t.phase == PhaseIdle && t.deps.IdleTime != nil
}

// startSmart begins the activity monitor. It does nothing when disabled.
func (t *WorkTimer) startSmart() {
	t.mu.Lock()
	if !t.smartWantedNoLock() {
		t.mu.Unlock()
		return
	}
	if t.stopSmart != nil {
		t.mu.Unlock()
		return
	}
	t.stopSmart = make(chan struct{})
	stop := t.stopSmart
	t.activeSince = time.Time{}
	t.mu.Unlock()
	go t.smartLoop(stop)
}

func (t *WorkTimer) smartWantedNoLock() bool {
	return t.smartEnabled && t.phase == PhaseIdle && t.deps.IdleTime != nil
}

func (t *WorkTimer) stopSmartMonitor() {
	t.mu.Lock()
	stop := t.stopSmart
	t.stopSmart = nil
	t.activeSince = time.Time{}
	t.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (t *WorkTimer) smartLoop(stop chan struct{}) {
	ticker := time.NewTicker(t.SmartPoll)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			if !t.smartWantedNoLock() {
				t.stopSmart = nil
				t.mu.Unlock()
				return
			}
			idle := t.deps.IdleTime()
			if idle < t.ActiveBelow {
				if t.activeSince.IsZero() {
					t.activeSince = t.now()
				} else if t.now().Sub(t.activeSince) >= t.smartThreshold {
					// Reminded once: stop until the timer runs and ends again.
					t.stopSmart = nil
					t.mu.Unlock()
					t.fire(t.hooks.Blink, true)
					t.fire0(t.hooks.SmartReminder)
					return
				}
			} else {
				t.activeSince = time.Time{}
			}
			t.mu.Unlock()
		}
	}
}

// RestoreOf rebuilds the timer from what was persisted at quit, as the
// Swift app did: a run whose time had not expired comes back paused,
// awaiting a manual start; a run that expired while the app was away is
// simply reset (no rest is forced for time already gone). A timer the
// user paused also comes back paused.
func (t *WorkTimer) Restore(saved SnapshotPersisted, workDuration time.Duration) Snapshot {
	t.mu.Lock()
	t.phase = PhaseIdle
	t.remaining, t.total, t.startedAt = 0, 0, time.Time{}
	t.mu.Unlock()

	switch {
	case saved.Running && !saved.Start.IsZero():
		elapsed := t.now().Sub(saved.Start)
		if elapsed < saved.Remaining {
			remain := saved.Remaining - elapsed
			if remain <= 0 {
				t.firePersist(Snapshot{}, time.Time{}, false)
				return t.Snapshot()
			}
			t.mu.Lock()
			t.phase = PhasePaused
			t.total = workDuration
			t.remaining = remain
			snap := t.snapshotLocked()
			t.mu.Unlock()
			t.firePersist(snap, saved.Start, false)
			t.fireS(t.hooks.Change, snap)
			return snap
		}
		t.firePersist(Snapshot{}, time.Time{}, false)
		return t.Snapshot()
	case saved.Remaining > 0:
		// Paused by the user at quit: bring back what was left.
		t.mu.Lock()
		t.phase = PhasePaused
		t.total = workDuration
		t.remaining = saved.Remaining
		snap := t.snapshotLocked()
		t.mu.Unlock()
		t.fireS(t.hooks.Change, snap)
		return snap
	default:
		return t.Snapshot()
	}
}

// SnapshotPersisted is what timer-state.json held, in plain types for the
// core package to reason about.
type SnapshotPersisted struct {
	Start     time.Time
	Running   bool
	Remaining time.Duration
}
