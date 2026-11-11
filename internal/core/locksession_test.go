package core

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLockSessionRunsOut(t *testing.T) {
	var mu sync.Mutex
	ticks, done := 0, false
	l := NewLockSession(LockHooks{
		Tick: func(time.Duration) { mu.Lock(); ticks++; mu.Unlock() },
		Done: func() { mu.Lock(); done = true; mu.Unlock() },
	})
	l.TickEvery = 5 * time.Millisecond
	l.Start(60*time.Millisecond, time.Now())

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		finished := done
		mu.Unlock()
		if finished || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	mu.Lock()
	finished, count := done, ticks
	mu.Unlock()
	if !finished {
		t.Fatal("session did not run out")
	}
	if count == 0 {
		t.Error("no ticks")
	}
	if l.Active() {
		t.Error("still active after running out")
	}
}

func TestLockSessionStopEarly(t *testing.T) {
	var done atomic.Bool
	l := NewLockSession(LockHooks{Done: func() { done.Store(true) }})
	l.TickEvery = 5 * time.Millisecond
	l.Start(10*time.Second, time.Now())
	if !l.Active() {
		t.Fatal("not active after start")
	}
	l.Stop()
	if l.Active() {
		t.Fatal("active after stop")
	}
	if l.Remaining() != 0 {
		t.Error("remaining after stop")
	}
	time.Sleep(50 * time.Millisecond)
	if done.Load() {
		t.Error("Done fired after an early stop")
	}
}

func TestMessages(t *testing.T) {
	msg := PickBreakMessage([]string{"工作了 {{duration}}，休息！"}, false, 25*time.Minute)
	if msg != "工作了 25分钟，休息！" {
		t.Errorf("break message = %q", msg)
	}
	msg = PickBreakMessage(nil, false, 90*time.Minute)
	if !strings.Contains(msg, "1小时30分钟") {
		t.Errorf("long duration message = %q", msg)
	}
	msg = PickBreakMessage(nil, true, 25*time.Minute)
	if !strings.Contains(msg, "25分钟") {
		t.Errorf("motivational message = %q", msg)
	}
	msg = HydrationMessageWithStats([]string{"喝水！"}, 3, 8)
	if !strings.Contains(msg, "喝水！") || !strings.Contains(msg, "进度 3/8") {
		t.Errorf("hydration message = %q", msg)
	}
	msg = HydrationMessageWithStats([]string{"喝水！"}, 8, 8)
	if !strings.Contains(msg, "已完成目标") {
		t.Errorf("goal message = %q", msg)
	}
}

func TestFormatClock(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "00:00",
		59 * time.Second: "00:59",
		25 * time.Minute: "25:00",
		90 * time.Minute: "1:30:00",
		-5 * time.Second: "00:00",
	}
	for d, want := range cases {
		if got := FormatClock(d); got != want {
			t.Errorf("FormatClock(%v) = %q, want %q", d, got, want)
		}
	}
}
