// Command preview renders Restify's views headlessly to PNG files, for
// looking at the interface without launching the app.
//
//	go run ./tools/preview -view popover -out /tmp/restify
//
// Every view renders in the light and the dark appearance, and the panel
// also over the popover material it shows in on screen.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"restify/internal/core"
	"restify/internal/gui"
	"restify/internal/store"
)

// ---- fakes ----

type fakePopover struct {
	snap     core.Snapshot
	today    int
	goal     int
	feedback string
	feedAt   time.Time
}

func (f *fakePopover) Timer() core.Snapshot               { return f.snap }
func (f *fakePopover) Settings() store.Settings           { return store.DefaultSettings() }
func (f *fakePopover) TodayCount() int                    { return f.today }
func (f *fakePopover) DailyGoal() int                     { return f.goal }
func (f *fakePopover) RecordDrink() (bool, int)           { return true, 0 }
func (f *fakePopover) DrinkFeedback() (string, time.Time) { return f.feedback, f.feedAt }
func (f *fakePopover) ToggleTimer()                       {}
func (f *fakePopover) ResetTimer()                        {}
func (f *fakePopover) SkipBreak()                         {}
func (f *fakePopover) OpenSettings()                      {}
func (f *fakePopover) OpenStats()                         {}
func (f *fakePopover) Version() string                    { return "1.0.0" }

type fakeSettings struct{ set store.Settings }

func (f *fakeSettings) Settings() store.Settings         { return f.set }
func (f *fakeSettings) SetSetting(func(*store.Settings)) {}
func (f *fakeSettings) TodayCount() int                  { return 3 }
func (f *fakeSettings) ImportSound(kind string) string   { return "" }
func (f *fakeSettings) TestSound(kind string)            {}
func (f *fakeSettings) SoundPreviewState(kind string) (bool, bool) {
	return false, false
}
func (f *fakeSettings) AddMessage(kind, text string)            {}
func (f *fakeSettings) RemoveMessage(kind string, i int)        {}
func (f *fakeSettings) ResetMessages(kind string)               {}
func (f *fakeSettings) SendTestNotification()                   {}
func (f *fakeSettings) NotificationNotice() (string, time.Time) { return "", time.Time{} }
func (f *fakeSettings) ExportData() (string, error)             { return "", nil }
func (f *fakeSettings) ClearAllData() bool                      { return false }
func (f *fakeSettings) ApplyLaunchAtLogin(bool)                 {}
func (f *fakeSettings) Version() string                         { return "1.0.0" }
func (f *fakeSettings) OpenStats()                              {}

type fakeStats struct{}

func (fakeStats) TodayCount() int       { return 3 }
func (fakeStats) DailyGoal() int        { return 8 }
func (fakeStats) ConsecutiveDays() int  { return 4 }
func (fakeStats) HighestRecord() int    { return 9 }
func (fakeStats) AverageDaily() float64 { return 6.2 }
func (fakeStats) Badges() (bool, bool, int, bool) {
	return true, false, 12, true
}
func (fakeStats) WeekStats() []store.DayCount {
	var days []store.DayCount
	now := time.Now()
	counts := []int{5, 8, 3, 7, 9, 6, 3}
	for i := 6; i >= 0; i-- {
		days = append(days, store.DayCount{Day: now.AddDate(0, 0, -i), Count: counts[6-i]})
	}
	return days
}
func (fakeStats) MonthStats() []store.DayCount { return fakeStats{}.WeekStats() }

type fakeAlert struct{}

func (fakeAlert) HydrationMessage() string {
	return "工作辛苦了，喝口水休息一下，保持好状态！"
}
func (fakeAlert) TodayCount() int      { return 3 }
func (fakeAlert) DailyGoal() int       { return 8 }
func (fakeAlert) RecordDrinkAndClose() {}
func (fakeAlert) DrinkDenial() int     { return 0 }
func (fakeAlert) SnoozeAndClose()      {}

type fakeLock struct{ remaining, total time.Duration }

func (f fakeLock) RestRemaining() time.Duration { return f.remaining }
func (f fakeLock) RestTotal() time.Duration     { return f.total }
func (f fakeLock) EndRest()                     {}

// ---- rendering ----

type target struct {
	name string
	view func(c *ui.Context)
	w, h int
	dark bool
	// click is the label to click before the capture, to reach a page.
	click string
	// setup runs once the tester exists, to put the model in a state that
	// needs the tester's clock.
	setup func(tt *ui.Tester)
}

func targets() []target {
	idle := &fakePopover{snap: core.Snapshot{Phase: core.PhaseIdle}, today: 3, goal: 8}
	running := &fakePopover{
		snap:  core.Snapshot{Phase: core.PhaseRunning, Remaining: 18*time.Minute + 32*time.Second, Total: 25 * time.Minute},
		today: 3, goal: 8,
	}
	goalReached := &fakePopover{
		snap:  core.Snapshot{Phase: core.PhaseRunning, Remaining: 90 * time.Second, Total: 25 * time.Minute},
		today: 8, goal: 8,
	}
	feedback := &fakePopover{
		snap: core.Snapshot{Phase: core.PhaseIdle}, today: 3, goal: 8,
		feedback: "已记录，今日 3/8 杯", feedAt: time.Now(),
	}
	settings := &fakeSettings{set: store.DefaultSettings()}

	var out []target
	for _, tc := range []struct {
		name string
		m    *fakePopover
		h    int
	}{
		{"popover-idle", idle, gui.PanelHeight(core.PhaseIdle, false)},
		{"popover-running", running, gui.PanelHeight(core.PhaseRunning, false)},
		{"popover-goal", goalReached, gui.PanelHeight(core.PhaseRunning, false)},
		{"popover-feedback", feedback, gui.PanelHeight(core.PhaseIdle, true)},
	} {
		mkFeedback := feedback
		_ = mkFeedback
		pop := &gui.Popover{Model: tc.m}
		var setup func(tt *ui.Tester)
		if tc.name == "popover-feedback" {
			setup = func(tt *ui.Tester) { tc.m.feedAt = tt.Now() }
		}
		out = append(out, target{tc.name, pop.View, 340, tc.h, false, "", setup})
		out = append(out, target{tc.name + "-dark", pop.View, 340, tc.h, true, "", setup})
	}
	for _, dark := range []bool{false, true} {
		for _, page := range []struct{ id, label string }{
			{"work", ""}, {"hydration", "喝水"}, {"general", "通用"}, {"about", "关于"},
		} {
			name := "settings-" + page.id
			if dark {
				name += "-dark"
			}
			// A fresh window per capture: the page is its state, and it
			// would carry from one target into the next.
			win := &gui.SettingsWindow{Model: settings}
			out = append(out, target{name, win.View, 720, 560, dark, page.label, nil})
		}
	}
	st := &gui.StatsWindow{Model: fakeStats{}}
	for _, dark := range []bool{false, true} {
		name := "stats"
		if dark {
			name += "-dark"
		}
		out = append(out, target{name, st.View, 680, 640, dark, "", nil})
	}
	al := &gui.HydrationAlert{Model: fakeAlert{}}
	for _, dark := range []bool{false, true} {
		name := "alert"
		if dark {
			name += "-dark"
		}
		out = append(out, target{name, al.View, 380, 460, dark, "", nil})
	}
	lock := &gui.LockOverlay{Model: fakeLock{remaining: 4*time.Minute + 12*time.Second, total: 5 * time.Minute}}
	out = append(out, target{"rest", lock.View, 900, 620, false, "", nil})
	return out
}

// measure renders the panel until its height tracker settles and reports
// the height the content asks for in each state.
func measure() {
	cases := []struct {
		name string
		m    *fakePopover
	}{
		{"idle", &fakePopover{snap: core.Snapshot{Phase: core.PhaseIdle}, today: 0, goal: 8}},
		{"idle-feedback", &fakePopover{snap: core.Snapshot{Phase: core.PhaseIdle}, today: 3, goal: 8, feedback: "已记录，今日 3/8 杯"}},
		{"running", &fakePopover{snap: core.Snapshot{Phase: core.PhaseRunning, Remaining: 25 * time.Minute, Total: 25 * time.Minute}, today: 3, goal: 8}},
		{"running-goal", &fakePopover{snap: core.Snapshot{Phase: core.PhaseRunning, Remaining: 90 * time.Second, Total: 25 * time.Minute}, today: 8, goal: 8}},
	}
	for _, tc := range cases {
		settled := 0
		p := &gui.Popover{Model: tc.m, OnHeight: func(h int) { settled = h }}
		tt := ui.NewTester(p.View, 340, 700)
		if tc.m.feedback != "" {
			tc.m.feedAt = tt.Now()
		}
		for range 80 {
			tt.Advance(50 * time.Millisecond)
		}
		fmt.Printf("%-16s settles at %d\n", tc.name, settled)
	}
}

func main() {
	out := flag.String("out", "/tmp/restify-preview", "directory for the PNG files")
	measureOnly := flag.Bool("measure", false, "print the heights the panel settles at and exit")
	only := flag.String("view", "", "render only this view (prefix match)")
	dark := flag.Bool("dark", false, "render only the dark appearance")
	flag.Parse()
	if *measureOnly {
		measure()
		return
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, t := range targets() {
		if *only != "" && !hasPrefix(t.name, *only) {
			continue
		}
		if *dark && !hasSuffix(t.name, "-dark") {
			continue
		}
		tt := ui.NewTester(t.view, t.w, t.h)
		tt.SetDark(t.dark)
		tt.Advance(700 * time.Millisecond)
		if t.setup != nil {
			t.setup(tt)
			tt.Advance(300 * time.Millisecond)
		}
		if t.click != "" {
			tt.Frame()
			if err := tt.Click(t.click); err != nil {
				fmt.Println("click", t.click, ":", err)
			}
			tt.Advance(300 * time.Millisecond)
		}
		tt.Frame()

		// Report the last row the content paints, for the panel's height
		// constants and for spotting trailing space.
		fmt.Printf("%-32s content bottom %d of %d\n", t.name, contentBottom(tt.Image()), t.h)

		path := filepath.Join(*out, t.name+".png")
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Println(path)
	}
}

// contentBottom is the last row of an image that differs from its
// background, the color at its top-left corner.
func contentBottom(img image.Image) int {
	b := img.Bounds()
	bgR, bgG, bgB, bgA := img.At(b.Min.X, b.Min.Y).RGBA()
	last := b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if r != bgR || g != bgG || bl != bgB || a != bgA {
				last = y
				break
			}
		}
	}
	return last
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func hasSuffix(s, p string) bool { return len(s) >= len(p) && s[len(s)-len(p):] == p }
