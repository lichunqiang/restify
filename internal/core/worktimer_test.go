package core

import (
	"sync"
	"testing"
	"time"
)

// recordingHooks collects what a WorkTimer did, safely.
type recordingHooks struct {
	mu      sync.Mutex
	ticks   []Snapshot
	changes []Phase
	breaks  int
	persist []Snapshot
	idle    int
	smart   int
	blink   []bool
}

func (r *recordingHooks) hooks() WorkHooks {
	return WorkHooks{
		Tick:   func(s Snapshot) { r.mu.Lock(); r.ticks = append(r.ticks, s); r.mu.Unlock() },
		Change: func(s Snapshot) { r.mu.Lock(); r.changes = append(r.changes, s.Phase); r.mu.Unlock() },
		Break:  func(Snapshot) { r.mu.Lock(); r.breaks++; r.mu.Unlock() },
		Persist: func(s Snapshot, _ time.Time, _ bool) {
			r.mu.Lock()
			r.persist = append(r.persist, s)
			r.mu.Unlock()
		},
		IdlePause:     func() { r.mu.Lock(); r.idle++; r.mu.Unlock() },
		SmartReminder: func() { r.mu.Lock(); r.smart++; r.mu.Unlock() },
		Blink:         func(on bool) { r.mu.Lock(); r.blink = append(r.blink, on); r.mu.Unlock() },
	}
}

func (r *recordingHooks) counts() (breaks, idle, smart int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.breaks, r.idle, r.smart
}

func (r *recordingHooks) lastBlink() (bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.blink) == 0 {
		return false, false
	}
	return r.blink[len(r.blink)-1], true
}

func (r *recordingHooks) tickCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ticks)
}

func fastTimer(idle func() time.Duration, rh *recordingHooks) *WorkTimer {
	t := NewWorkTimer(WorkDeps{IdleTime: idle}, rh.hooks())
	t.TickEvery = 5 * time.Millisecond
	t.PersistEvery = 50 * time.Millisecond
	t.IdlePoll = 10 * time.Millisecond
	t.IdleThreshold = 300 * time.Millisecond
	t.SmartPoll = 10 * time.Millisecond
	t.ActiveBelow = 60 * time.Millisecond
	t.smartThreshold = 200 * time.Millisecond
	return t
}

func TestWorkTimerRunsToBreak(t *testing.T) {
	rh := &recordingHooks{}
	wt := fastTimer(nil, rh)
	wt.Start(60 * time.Millisecond) // 12 ticks at 5ms

	deadline := time.Now().Add(2 * time.Second)
	for wt.Snapshot().Phase != PhaseIdle && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := wt.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase = %v, want idle", got)
	}
	if breaks, _, _ := rh.counts(); breaks != 1 {
		t.Errorf("breaks = %d, want 1", breaks)
	}
	if rh.tickCount() == 0 {
		t.Error("no ticks recorded")
	}
}

func TestWorkTimerPauseResume(t *testing.T) {
	rh := &recordingHooks{}
	wt := fastTimer(nil, rh)
	wt.Start(10 * time.Second)

	deadline := time.Now().Add(time.Second)
	for wt.Snapshot().Remaining >= 10*time.Second && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	wt.Pause()
	snap := wt.Snapshot()
	if snap.Phase != PhasePaused || snap.Remaining <= 0 || snap.Remaining >= 10*time.Second {
		t.Fatalf("paused snapshot = %+v", snap)
	}
	// No ticks while paused.
	n := len(rh.ticks)
	time.Sleep(50 * time.Millisecond)
	if len(rh.ticks) != n {
		t.Error("ticked while paused")
	}

	wt.Resume()
	if wt.Snapshot().Phase != PhaseRunning {
		t.Fatal("not running after resume")
	}
	wt.Pause()
	wt.Reset()
	if got := wt.Snapshot(); got.Phase != PhaseIdle || got.Remaining != 0 {
		t.Fatalf("reset snapshot = %+v", got)
	}
}

func TestWorkTimerIdleAutoPause(t *testing.T) {
	rh := &recordingHooks{}
	idleFor := func() time.Duration { return time.Hour } // idle forever
	wt := fastTimer(idleFor, rh)
	wt.Start(10 * time.Second)

	deadline := time.Now().Add(2 * time.Second)
	for wt.Snapshot().Phase != PhasePaused && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if wt.Snapshot().Phase != PhasePaused {
		t.Fatal("idle did not pause the timer")
	}
	if _, idle, _ := rh.counts(); idle != 1 {
		t.Errorf("idle pauses = %d, want 1", idle)
	}
}

func TestSmartReminder(t *testing.T) {
	rh := &recordingHooks{}
	idleFor := func() time.Duration { return 0 } // always active
	wt := fastTimer(idleFor, rh)
	wt.SetSmartReminder(true, 200*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, _, smart := rh.counts(); smart > 0 || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, _, smart := rh.counts(); smart != 1 {
		t.Fatalf("smart reminders = %d, want 1", smart)
	}
	// Blink was asked on for the nudge.
	if on, ok := rh.lastBlink(); !ok || !on {
		t.Error("blink not on for the nudge")
	}

	// Starting the timer stops the blink.
	wt.Start(5 * time.Second)
	if on, ok := rh.lastBlink(); !ok || on {
		t.Error("blink not off after start")
	}
}

func TestSmartReminderQuietWhenIdle(t *testing.T) {
	rh := &recordingHooks{}
	idleFor := func() time.Duration { return time.Hour } // user away
	wt := fastTimer(idleFor, rh)
	wt.SetSmartReminder(true, 200*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	if _, _, smart := rh.counts(); smart != 0 {
		t.Errorf("smart reminders = %d, want 0 for an idle user", smart)
	}
}

func TestRestoreRunningUnexpired(t *testing.T) {
	rh := &recordingHooks{}
	wt := fastTimer(nil, rh)
	wt.Restore(SnapshotPersisted{
		Start:     time.Now().Add(-5 * time.Minute),
		Running:   true,
		Remaining: 25 * time.Minute,
	}, 25*time.Minute)

	snap := wt.Snapshot()
	if snap.Phase != PhasePaused {
		t.Fatalf("phase = %v, want paused", snap.Phase)
	}
	if want := 20 * time.Minute; snap.Remaining > want || snap.Remaining < want-2*time.Minute {
		t.Errorf("remaining = %v, want about %v", snap.Remaining, want)
	}
}

func TestRestoreRunningExpired(t *testing.T) {
	rh := &recordingHooks{}
	wt := fastTimer(nil, rh)
	wt.Restore(SnapshotPersisted{
		Start:     time.Now().Add(-time.Hour),
		Running:   true,
		Remaining: 25 * time.Minute,
	}, 25*time.Minute)
	if got := wt.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase = %v, want idle", got)
	}
	if breaks, _, _ := rh.counts(); breaks != 0 {
		t.Errorf("breaks = %d, want 0: no rest for time already gone", breaks)
	}
}

func TestRestorePaused(t *testing.T) {
	wt := fastTimer(nil, &recordingHooks{})
	wt.Restore(SnapshotPersisted{Remaining: 7 * time.Minute}, 25*time.Minute)
	snap := wt.Snapshot()
	if snap.Phase != PhasePaused || snap.Remaining != 7*time.Minute {
		t.Fatalf("snapshot = %+v, want paused with 7m", snap)
	}
}
