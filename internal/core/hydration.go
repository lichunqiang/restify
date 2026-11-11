package core

import (
	"sync"
	"time"

	"restify/internal/store"
)

// HydrationHooks report what the hydration loop does. They run on the
// loop's goroutine or on the goroutine that called a control.
type HydrationHooks struct {
	Remind    func()                   // time to remind the user to drink
	Celebrate func(goal int)           // the day's goal was just reached
	NewDay    func(yesterdayCount int) // midnight passed
	Denied    func(waitSecs int)       // a record came too soon after the last
	Change    func()                   // the day's count changed
}

// Hydration runs the drink reminders: a repeating tick that only reminds
// when the goal is unmet and the last drink is older than the interval,
// a midnight reset, and the drink log itself.
type Hydration struct {
	recs  *store.RecordsStore
	set   *store.SettingsStore
	hooks HydrationHooks

	// MinRecordInterval throttles double records; tests shrink it.
	MinRecordInterval time.Duration

	mu         sync.Mutex
	today      int
	todayDay   time.Time
	lastRecord time.Time
	stopTick   chan struct{}
	midnight   *time.Timer
}

// NewHydration makes the loop over the given stores.
func NewHydration(recs *store.RecordsStore, set *store.SettingsStore, hooks HydrationHooks) *Hydration {
	return &Hydration{
		recs:              recs,
		set:               set,
		hooks:             hooks,
		MinRecordInterval: 30 * time.Second,
	}
}

// fire runs a hook that may be nil.
func (h *Hydration) fire(fn func()) {
	if fn != nil {
		fn()
	}
}

// Start loads today's count and starts the reminder tick and the midnight
// reset. Call once, after settings exist.
func (h *Hydration) Start(now time.Time) {
	h.mu.Lock()
	h.todayDay = store.DayKey(now)
	h.today = h.recs.CountFor(now)
	h.mu.Unlock()

	h.restartTick()
	h.scheduleMidnight(now)
	h.fire(h.hooks.Change)
}

// Stop halts every timer.
func (h *Hydration) Stop() {
	h.mu.Lock()
	stop := h.stopTick
	h.stopTick = nil
	if h.midnight != nil {
		h.midnight.Stop()
		h.midnight = nil
	}
	h.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// Reschedule applies a changed interval or switch: call after the settings
// change.
func (h *Hydration) Reschedule() {
	h.restartTick()
}

func (h *Hydration) restartTick() {
	h.mu.Lock()
	if h.stopTick != nil {
		close(h.stopTick)
	}
	s := h.set.Get()
	if !s.EnableReminder {
		h.stopTick = nil
		h.mu.Unlock()
		return
	}
	h.stopTick = make(chan struct{})
	stop := h.stopTick
	h.mu.Unlock()
	go h.tickLoop(stop)
}

func (h *Hydration) tickLoop(stop chan struct{}) {
	for {
		h.mu.Lock()
		interval := h.set.Get().ReminderInterval
		h.mu.Unlock()
		if interval <= 0 {
			interval = time.Hour
		}
		select {
		case <-stop:
			return
		case <-time.After(interval):
			h.CheckAndRemind(time.Now())
		}
	}
}

// CheckAndRemind reminds when the goal is unmet and the last drink is at
// least one interval old. It reports whether it reminded.
func (h *Hydration) CheckAndRemind(now time.Time) bool {
	h.rolloverIfNeeded(now)

	s := h.set.Get()
	if !s.EnableReminder {
		return false
	}
	h.mu.Lock()
	today := h.today
	h.mu.Unlock()
	if today >= s.DailyGoal {
		return false
	}
	last := h.recs.LastDrink()
	if !last.IsZero() && now.Sub(last) < s.ReminderInterval {
		return false
	}

	h.fire(h.hooks.Remind)
	return true
}

// RecordDrink logs one drink. It reports false and how long to wait when
// it came too soon after the last.
func (h *Hydration) RecordDrink(now time.Time) (bool, int) {
	h.rolloverIfNeeded(now)

	h.mu.Lock()
	if !h.lastRecord.IsZero() {
		wait := h.MinRecordInterval - now.Sub(h.lastRecord)
		if wait > 0 {
			h.mu.Unlock()
			secs := int((wait + time.Second - 1) / time.Second)
			h.fire(func() {
				if h.hooks.Denied != nil {
					h.hooks.Denied(secs)
				}
			})
			return false, secs
		}
	}
	h.today++
	h.lastRecord = now
	today := h.today
	h.mu.Unlock()

	if err := h.recs.RecordDrink(now, today); err != nil {
		// The count is in memory either way; the file retries on the next
		// record or on quit.
		_ = err
	}

	h.fire(h.hooks.Change)
	if today == h.set.Get().DailyGoal {
		h.fire(func() {
			if h.hooks.Celebrate != nil {
				h.hooks.Celebrate(today)
			}
		})
	}
	return true, 0
}

// TodayCount reports today's cumulative count, rolling the day over when
// midnight passed since the last look.
func (h *Hydration) TodayCount(now time.Time) int {
	h.rolloverIfNeeded(now)
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.today
}

// Progress is today's count over the goal, clamped to 1.
func (h *Hydration) Progress(now time.Time) float64 {
	s := h.set.Get()
	if s.DailyGoal <= 0 {
		return 0
	}
	p := float64(h.TodayCount(now)) / float64(s.DailyGoal)
	if p > 1 {
		p = 1
	}
	return p
}

// rolloverIfNeeded resets the day when midnight passed since the last
// touch, standing in for a missed midnight timer (sleep, suspend).
func (h *Hydration) rolloverIfNeeded(now time.Time) {
	day := store.DayKey(now)
	h.mu.Lock()
	same := h.todayDay.Equal(day)
	h.mu.Unlock()
	if same {
		return
	}

	h.mu.Lock()
	yesterday := h.today
	h.todayDay = day
	h.today = h.recs.CountFor(now)
	h.mu.Unlock()

	if yesterday > 0 {
		h.fire(func() {
			if h.hooks.NewDay != nil {
				h.hooks.NewDay(yesterday)
			}
		})
	}
	h.fire(h.hooks.Change)
}

// scheduleMidnight arranges the next day's reset, re-arming itself.
func (h *Hydration) scheduleMidnight(now time.Time) {
	next := store.DayKey(now).AddDate(0, 0, 1)
	delay := next.Sub(now)
	if delay <= 0 {
		delay = time.Second
	}
	h.mu.Lock()
	if h.midnight != nil {
		h.midnight.Stop()
	}
	h.midnight = time.AfterFunc(delay, func() {
		h.rolloverIfNeeded(time.Now())
		h.scheduleMidnight(time.Now())
	})
	h.mu.Unlock()
}
