package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TimerState is the work timer's persisted state, written every minute
// while it runs and cleared when the work period ends.
type TimerState struct {
	StartTime time.Time     `json:"startTime"` // zero when none
	Running   bool          `json:"running"`
	Remaining time.Duration `json:"remaining"`
}

// TimerStateStore keeps TimerState in timer-state.json. Safe from any
// goroutine.
type TimerStateStore struct {
	mu   sync.Mutex
	path string
	s    TimerState
}

// OpenTimerState loads timer-state.json in dir, creating it when absent.
func OpenTimerState(dir string) (*TimerStateStore, error) {
	ts := &TimerStateStore{path: filepath.Join(dir, "timer-state.json")}
	data, err := os.ReadFile(ts.path)
	if os.IsNotExist(err) {
		return ts, nil
	}
	if err != nil {
		return ts, err
	}
	return ts, json.Unmarshal(data, &ts.s)
}

// Get returns the persisted state.
func (ts *TimerStateStore) Get() TimerState {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.s
}

// Save writes the state whole.
func (ts *TimerStateStore) Save(s TimerState) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.s = s
	return writeAtomic(ts.path, mustJSON(s))
}

// Clear removes the persisted state (the timer is over or idle).
func (ts *TimerStateStore) Clear() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.s = TimerState{}
	return writeAtomic(ts.path, mustJSON(ts.s))
}

func mustJSON(v any) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}
