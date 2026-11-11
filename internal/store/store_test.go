package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := DefaultSettings()
	if got := st.Get(); got.WorkDuration != d.WorkDuration || got.DailyGoal != d.DailyGoal ||
		len(got.BreakMessageLibrary) != len(d.BreakMessageLibrary) || got.EnableAutoLock != d.EnableAutoLock {
		t.Fatalf("fresh store is not the defaults: %+v", got)
	}
	err = st.Update(func(s *Settings) {
		s.WorkDuration = 45 * time.Minute
		s.DailyGoal = 10
		s.ThemeMode = ThemeDark
	})
	if err != nil {
		t.Fatal(err)
	}

	st2, err := OpenSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := st2.Get()
	if got.WorkDuration != 45*time.Minute || got.DailyGoal != 10 || got.ThemeMode != ThemeDark {
		t.Fatalf("settings did not survive a reload: %+v", got)
	}
	if len(got.BreakMessageLibrary) != len(d.BreakMessageLibrary) {
		t.Error("message library lost")
	}

	// Files land in dir.
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRecordsCountAndPrune(t *testing.T) {
	rs, err := OpenRecords(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 3, 9, 0, 0, 0, time.Local)
	for i := 1; i <= 3; i++ {
		if err := rs.RecordDrink(day.Add(time.Duration(i)*time.Minute), i); err != nil {
			t.Fatal(err)
		}
	}
	if got := rs.CountFor(day); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	if rs.CountFor(day.AddDate(0, 0, 1)) != 0 {
		t.Error("next day counted")
	}
	if rs.LastDrink().IsZero() {
		t.Error("last drink not recorded")
	}

	// A record 31 days later prunes the old day out.
	later := day.AddDate(0, 0, 31)
	if err := rs.RecordDrink(later, 1); err != nil {
		t.Fatal(err)
	}
	if got := rs.CountFor(day); got != 0 {
		t.Errorf("pruned day still counted: %d", got)
	}
}

func TestRecordsStats(t *testing.T) {
	rs, _ := OpenRecords(t.TempDir())
	// Two days ago: 8 (goal). Yesterday: none. Today: 5.
	two := time.Now().AddDate(0, 0, -2)
	for i := 1; i <= 8; i++ {
		rs.RecordDrink(two.Add(time.Duration(i)*time.Minute), i)
	}
	today := time.Now()
	for i := 1; i <= 5; i++ {
		rs.RecordDrink(today.Add(time.Duration(i)*time.Minute), i)
	}

	week := rs.WeekStats(time.Now())
	if len(week) != 7 {
		t.Fatalf("week has %d days", len(week))
	}
	if week[6].Count != 5 || week[5].Count != 0 || week[4].Count != 8 {
		t.Errorf("week = %+v", week)
	}

	if got := rs.HighestRecord(); got != 8 {
		t.Errorf("highest = %d, want 8", got)
	}
	if got := rs.ConsecutiveDays(time.Now()); got != 1 {
		t.Errorf("streak = %d, want 1 (yesterday was dry)", got)
	}
	// Average over 7 days: (8+5)/7.
	avg := rs.AverageDaily(time.Now())
	if avg < 13.0/7.0-1e-9 || avg > 13.0/7.0+1e-9 {
		t.Errorf("average = %v, want %v", avg, 13.0/7.0)
	}

	goalDays, total := rs.Badges(8)
	if goalDays != 1 || total != 13 {
		t.Errorf("badges = %d goal days, %d drinks; want 1, 13", goalDays, total)
	}

	month := rs.MonthStats(time.Now())
	now := time.Now()
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	if len(month) != daysInMonth {
		t.Errorf("month has %d days, want %d", len(month), daysInMonth)
	}

	if err := rs.ClearDay(two); err != nil {
		t.Fatal(err)
	}
	if got := rs.CountFor(two); got != 0 {
		t.Error("clear day left records")
	}
	if err := rs.ClearAll(); err != nil {
		t.Fatal(err)
	}
	if got := rs.HighestRecord(); got != 0 {
		t.Error("clear all left records")
	}
}

func TestTimerState(t *testing.T) {
	dir := t.TempDir()
	ts, err := OpenTimerState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Get().Running {
		t.Error("fresh state is running")
	}
	start := time.Now()
	if err := ts.Save(TimerState{StartTime: start, Running: true, Remaining: 90 * time.Second}); err != nil {
		t.Fatal(err)
	}
	ts2, _ := OpenTimerState(dir)
	got := ts2.Get()
	if !got.Running || got.Remaining != 90*time.Second || !got.StartTime.Equal(start) {
		t.Fatalf("state = %+v", got)
	}
	if err := ts.Clear(); err != nil {
		t.Fatal(err)
	}
	if ts.Get().Running || ts.Get().Remaining != 0 {
		t.Error("clear left state")
	}
}

func TestDayKeyUsesLocalMidnight(t *testing.T) {
	tm := time.Date(2026, 11, 3, 23, 59, 59, 0, time.Local)
	key := DayKey(tm)
	if key.Hour() != 0 || key.Minute() != 0 || key.Day() != 3 {
		t.Fatalf("DayKey = %v", key)
	}
	if !key.Equal(DayKey(key.Add(1 * time.Second))) {
		t.Error("midnight second maps to a different day")
	}
}
