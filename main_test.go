package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"restify/internal/app"
)

// The tray panel renders against the real App wired to a temp data
// directory: a headless smoke test of the wiring the model methods use.
func TestAppPopoverWiring(t *testing.T) {
	a, err := app.NewForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.PopoverView(), 340, 520)
	tt.SetVibrancy(true)
	if !tt.HasText("Restify") || !tt.HasText("工作计时") || !tt.HasText("记录喝水") {
		t.Errorf("panel texts %q", tt.Texts())
	}
	if a.DailyGoal() != 8 {
		t.Errorf("default goal = %d", a.DailyGoal())
	}
	if a.TodayCount() != 0 {
		t.Errorf("fresh today count = %d", a.TodayCount())
	}
}

// The popup's message is chosen once for its showing: redraws, which the
// engines ask for every second, keep the same words.
func TestHydrationPopupMessageStable(t *testing.T) {
	a, err := app.NewForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := a.HydrationMessage()
	if first == "" {
		t.Fatal("empty message")
	}
	for i := 0; i < 20; i++ {
		if again := a.HydrationMessage(); again != first {
			t.Fatalf("message changed between renders: %q then %q", first, again)
		}
	}
}

// A record from the popup that comes too soon after the last leaves the
// denial readable, instead of closing as if it had been counted.
func TestHydrationPopupRecordDenial(t *testing.T) {
	dir := t.TempDir()
	// Sound off: the app's shell is headless here, and a beep would wait
	// for an event loop that never runs.
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"enableSound": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := app.NewForTest(dir)
	if err != nil {
		t.Fatal(err)
	}

	if a.DrinkDenial() != 0 {
		t.Fatalf("denial without a record = %d", a.DrinkDenial())
	}
	a.RecordDrinkAndClose()
	if a.TodayCount() != 1 {
		t.Fatalf("first record counted %d", a.TodayCount())
	}
	a.RecordDrinkAndClose()
	if a.TodayCount() != 1 {
		t.Fatalf("denied record still counted %d", a.TodayCount())
	}
	if wait := a.DrinkDenial(); wait <= 0 || wait > 30 {
		t.Fatalf("denial wait = %d", wait)
	}
}
