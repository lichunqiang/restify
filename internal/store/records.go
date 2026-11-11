package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is one drink: when it happened and the day's cumulative count
// after it, as the Swift app stored them.
type Record struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

// DayCount pairs a calendar day with its drink count.
type DayCount struct {
	Day   time.Time // the day at local midnight
	Count int
}

// recordsFile is the JSON written to disk.
type recordsFile struct {
	Records   []Record   `json:"records"`
	LastDrink *time.Time `json:"lastDrink,omitempty"`
}

// RecordsStore keeps the drink history in hydration-records.json. Only the
// last 30 days are kept. Safe from any goroutine.
type RecordsStore struct {
	mu    sync.Mutex
	path  string
	recs  []Record
	last  time.Time // zero when never
}

// OpenRecords loads hydration-records.json in dir, creating it when absent.
func OpenRecords(dir string) (*RecordsStore, error) {
	rs := &RecordsStore{path: filepath.Join(dir, "hydration-records.json")}
	data, err := os.ReadFile(rs.path)
	switch {
	case os.IsNotExist(err):
		return rs, nil
	case err != nil:
		return rs, err
	}
	var f recordsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return rs, err
	}
	rs.recs, rs.last = f.Records, time.Time{}
	if f.LastDrink != nil {
		rs.last = *f.LastDrink
	}
	return rs, nil
}

// DayKey truncates t to its local calendar day.
func DayKey(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// RecordDrink appends a record of count for at, keeps 30 days of history
// and marks the last drink.
func (rs *RecordsStore) RecordDrink(at time.Time, count int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.recs = append(rs.recs, Record{Timestamp: at, Count: count})
	cutoff := at.AddDate(0, 0, -30)
	kept := rs.recs[:0]
	for _, r := range rs.recs {
		if r.Timestamp.After(cutoff) {
			kept = append(kept, r)
		}
	}
	rs.recs = kept
	rs.last = at
	return rs.saveLocked()
}

// CountFor returns the day's cumulative count: the last record of that day,
// zero when the day has none.
func (rs *RecordsStore) CountFor(day time.Time) int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.countForLocked(DayKey(day))
}

func (rs *RecordsStore) countForLocked(day time.Time) int {
	count := 0
	for _, r := range rs.recs {
		if DayKey(r.Timestamp).Equal(day) {
			count = r.Count // records are append-only: the last one wins
		}
	}
	return count
}

// LastDrink reports when the last drink was recorded; zero when never.
func (rs *RecordsStore) LastDrink() time.Time {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.last
}

// WeekStats returns the last 7 days ending on now's day, oldest first.
func (rs *RecordsStore) WeekStats(now time.Time) []DayCount {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.statsLocked(func(i int) time.Time { return DayKey(now.AddDate(0, 0, -i)) }, 7)
}

// MonthStats returns the days of now's month, oldest first.
func (rs *RecordsStore) MonthStats(now time.Time) []DayCount {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	n := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	return rs.statsLocked(func(i int) time.Time { return first.AddDate(0, 0, i) }, n)
}

func (rs *RecordsStore) statsLocked(dayAt func(int) time.Time, n int) []DayCount {
	out := make([]DayCount, n)
	for i := range n {
		day := dayAt(n - 1 - i) // oldest first
		out[i] = DayCount{Day: day, Count: rs.countForLocked(day)}
	}
	return out
}

// ConsecutiveDays counts the days back from now's day that each have at
// least one drink (the day itself only counts once it has a record).
func (rs *RecordsStore) ConsecutiveDays(now time.Time) int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	day := DayKey(now)
	streak := 0
	for range 365 {
		if rs.countForLocked(day) == 0 {
			break
		}
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

// HighestRecord is the highest cumulative count ever recorded.
func (rs *RecordsStore) HighestRecord() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	highest := 0
	for _, r := range rs.recs {
		if r.Count > highest {
			highest = r.Count
		}
	}
	return highest
}

// GoalDays counts the days whose count reached goal, and TotalDrinks sums
// every day's count — the achievement badges' inputs.
func (rs *RecordsStore) Badges(goal int) (goalDays int, totalDrinks int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	perDay := map[time.Time]int{}
	for _, r := range rs.recs {
		perDay[DayKey(r.Timestamp)] = r.Count
	}
	for _, c := range perDay {
		if c >= goal {
			goalDays++
		}
		totalDrinks += c
	}
	return goalDays, totalDrinks
}

// AverageDaily is the mean count over the last 7 days, as the Swift app
// computed it (always over seven days, gaps included).
func (rs *RecordsStore) AverageDaily(now time.Time) float64 {
	week := rs.WeekStats(now)
	total := 0
	for _, d := range week {
		total += d.Count
	}
	return float64(total) / float64(len(week))
}

// ClearAll removes every record and the last-drink mark.
func (rs *RecordsStore) ClearAll() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.recs, rs.last = nil, time.Time{}
	return rs.saveLocked()
}

// ClearDay removes the records of one calendar day.
func (rs *RecordsStore) ClearDay(day time.Time) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	day = DayKey(day)
	kept := rs.recs[:0]
	for _, r := range rs.recs {
		if !DayKey(r.Timestamp).Equal(day) {
			kept = append(kept, r)
		}
	}
	rs.recs = kept
	return rs.saveLocked()
}

// All returns a copy of the whole history, oldest first.
func (rs *RecordsStore) All() []Record {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]Record(nil), rs.recs...)
}

func (rs *RecordsStore) saveLocked() error {
	f := recordsFile{Records: rs.recs}
	if !rs.last.IsZero() {
		last := rs.last
		f.LastDrink = &last
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(rs.path, data)
}
