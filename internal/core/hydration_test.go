package core

import (
	"sync"
	"testing"
	"time"

	"restify/internal/store"
)

func testStores(t *testing.T) (*store.RecordsStore, *store.SettingsStore) {
	t.Helper()
	dir := t.TempDir()
	recs, err := store.OpenRecords(dir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := store.OpenSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	return recs, set
}

func TestHydrationGates(t *testing.T) {
	recs, set := testStores(t)
	var mu sync.Mutex
	reminded := 0
	h := NewHydration(recs, set, HydrationHooks{Remind: func() { mu.Lock(); reminded++; mu.Unlock() }})
	now := time.Now()
	h.Start(now)

	// Fresh day, never drank: reminds.
	if !h.CheckAndRemind(now) {
		t.Error("should remind on a fresh day")
	}
	// But not twice inside one interval: the first remind is not a drink,
	// so time-since-last-drink still rules it out only after a drink. The
	// Swift app gated on lastDrinkTime; with no drink ever recorded it
	// would remind again — match that by only testing the goal gate here.
	reminded = 0

	// Goal met: no reminders. (Each drink a minute apart: the log
	// throttles records 30s apart.)
	for i := range set.Get().DailyGoal {
		h.RecordDrink(now.Add(time.Duration(i) * time.Minute))
	}
	if h.CheckAndRemind(now) {
		t.Error("should not remind once the goal is met")
	}
	if h.TodayCount(now) != set.Get().DailyGoal {
		t.Errorf("today = %d, want %d", h.TodayCount(now), set.Get().DailyGoal)
	}
}

func TestHydrationReminderGate(t *testing.T) {
	recs, set := testStores(t)
	s := set.Get()
	s.ReminderInterval = time.Hour
	_ = set.Update(func(x *store.Settings) { x.ReminderInterval = time.Hour })

	h := NewHydration(recs, set, HydrationHooks{})
	now := time.Now()
	h.Start(now)

	if ok, _ := h.RecordDrink(now); !ok {
		t.Fatal("first record denied")
	}
	// Too soon: denied.
	if ok, wait := h.RecordDrink(now.Add(5 * time.Second)); ok {
		t.Error("double record accepted")
	} else if wait <= 0 {
		t.Error("denied without a wait")
	}
	// After the last drink: no reminder inside the interval.
	if h.CheckAndRemind(now.Add(10 * time.Minute)) {
		t.Error("reminded inside the interval since the last drink")
	}
	// Past the interval and below goal: remind.
	if !h.CheckAndRemind(now.Add(90 * time.Minute)) {
		t.Error("no reminder past the interval")
	}
}

func TestHydrationMidnightRollover(t *testing.T) {
	recs, set := testStores(t)
	var newDays []int
	h := NewHydration(recs, set, HydrationHooks{NewDay: func(n int) { newDays = append(newDays, n) }})
	day1 := time.Date(2026, 11, 3, 20, 0, 0, 0, time.Local)
	h.Start(day1)
	h.RecordDrink(day1)
	h.RecordDrink(day1.Add(2 * time.Minute))

	day2 := time.Date(2026, 11, 4, 8, 0, 0, 0, time.Local)
	if got := h.TodayCount(day2); got != 0 {
		t.Fatalf("today = %d on the new day, want 0", got)
	}
	if len(newDays) != 1 || newDays[0] != 2 {
		t.Errorf("newDay callbacks = %v, want [2]", newDays)
	}
	// Yesterday's records survived.
	if got := recs.CountFor(day1); got != 2 {
		t.Errorf("yesterday count = %d, want 2", got)
	}
}

func TestHydrationCelebrate(t *testing.T) {
	recs, set := testStores(t)
	var celebrated int
	h := NewHydration(recs, set, HydrationHooks{Celebrate: func(int) { celebrated++ }})
	now := time.Now()
	h.Start(now)
	for i := range set.Get().DailyGoal {
		h.RecordDrink(now.Add(time.Duration(i) * time.Minute))
	}
	if celebrated != 1 {
		t.Errorf("celebrations = %d, want 1", celebrated)
	}
}
