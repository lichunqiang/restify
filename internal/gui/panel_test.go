package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"restify/internal/core"
)

// panelCase is one state that moves the panel's content height: the work
// card's two faces, the feedback line under the record button, and the
// tag the goal reached adds.
type panelCase struct {
	name     string
	snap     core.Snapshot
	today    int
	goal     int
	feedback bool
}

func panelCases() []panelCase {
	running := core.Snapshot{Phase: core.PhaseRunning, Remaining: 90 * time.Second, Total: 25 * time.Minute}
	return []panelCase{
		{"idle", core.Snapshot{Phase: core.PhaseIdle}, 0, 8, false},
		{"idle-feedback", core.Snapshot{Phase: core.PhaseIdle}, 3, 8, true},
		{"running", running, 3, 8, false},
		{"running-feedback", running, 3, 8, true},
		{"running-goal", running, 8, 8, false},
	}
}

// renderPanel lays the panel out in a case's state until its height
// settles at want — what PanelHeight names — and reports what it reached
// when the animations have all played out. The feedback cases start
// their toast fresh, as a drink just recorded.
func renderPanel(tc panelCase, want int) (settled int, tt *ui.Tester) {
	m := &fakePopoverModel{snap: tc.snap, today: tc.today, goal: tc.goal}
	p := &Popover{Model: m, OnHeight: func(h int) { settled = h }}
	tt = ui.NewTester(p.View, 340, want)
	tt.SetVibrancy(true)
	if tc.feedback {
		m.feedback, m.feedAt = "已记录，今日 3/8 杯", tt.Now()
		tt.Frame()
	}
	for range 40 { // the entrance, swap and line animations play out
		if settled == want {
			break
		}
		tt.Advance(100 * time.Millisecond)
	}
	return settled, tt
}

// TestPanelHeights lays the panel out in every state and checks the
// height its content settles at is the one PanelHeight names — so the
// constants cannot drift from the layout, and the window never opens
// clipping the footer nor leaving dead space under it.
func TestPanelHeights(t *testing.T) {
	for _, tc := range panelCases() {
		t.Run(tc.name, func(t *testing.T) {
			want := PanelHeight(tc.snap.Phase, tc.feedback)
			settled, _ := renderPanel(tc, want)
			if settled != want {
				t.Errorf("%s: content settles at %dpt but PanelHeight says %dpt — the constants drifted from the layout", tc.name, settled, want)
			}
		})
	}
}

// TestPanelRenders checks the two faces pixel by pixel: at the height
// PanelHeight names, nothing is clipped at the bottom and no more than
// the root padding is left empty under the content.
func TestPanelRenders(t *testing.T) {
	for _, tc := range panelCases() {
		t.Run(tc.name, func(t *testing.T) {
			_, tt := renderPanel(tc, PanelHeight(tc.snap.Phase, tc.feedback))

			// The panel paints an opaque surface, so the content is what
			// differs from that surface: sample it in the root padding.
			img := tt.Image()
			b := img.Bounds()
			bgR, bgG, bgB, bgA := img.At(b.Min.X+1, b.Min.Y+1).RGBA()
			bottom := 0
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					r, g, bl, a := img.At(x, y).RGBA()
					if r != bgR || g != bgG || bl != bgB || a != bgA {
						bottom = y
						break
					}
				}
			}
			empty := float64(b.Dy() - 1 - bottom)
			if empty > panelPad+12 {
				t.Errorf("%s: %vpt of empty space under the content (panel %dpt) — shrink the flyout height", tc.name, empty, b.Dy())
			}
			if bottom >= b.Dy()-2 {
				t.Errorf("%s: content touches the panel's bottom edge — clipped", tc.name)
			}

			if dir := os.Getenv("RESTIFY_GALLERY_DIR"); dir != "" {
				_ = os.MkdirAll(dir, 0o755)
				f, err := os.Create(filepath.Join(dir, "panel-"+tc.name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, img); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestPopoverTracksHeight follows the panel through its moves — a drink
// recorded, its feedback expiring, the timer started, the panel reopened
// — and checks the height it settles at each time, from a window that
// starts the wrong size: the window the panel lives in stays exactly as
// tall as its content, so the footer is never clipped and the bottom
// margin never changes. The heights it reports along a move step with
// the animations; what matters is where each one lands.
func TestPopoverTracksHeight(t *testing.T) {
	m := &fakePopoverModel{snap: core.Snapshot{Phase: core.PhaseIdle}, goal: 8}
	var heights []int
	p := &Popover{Model: m, OnHeight: func(h int) { heights = append(heights, h) }}
	tt := ui.NewTester(p.View, 340, 999) // the wrong height on purpose
	tt.SetVibrancy(true)

	// settle advances until the tracker's last report is want, as the
	// entrance, swap and line animations play their course.
	settle := func(want int) {
		t.Helper()
		for range 60 {
			tt.Advance(100 * time.Millisecond)
			if len(heights) > 0 && heights[len(heights)-1] == want {
				return
			}
		}
		t.Fatalf("height never settled at %d; reported %v", want, heights)
	}

	settle(panelIdleHeight) // opened idle
	if len(heights) != 1 || heights[0] != panelIdleHeight {
		t.Errorf("opening idle reported %v, want just %d", heights, panelIdleHeight)
	}

	// A recorded drink adds the feedback line for a moment.
	m.today = 1
	m.feedback, m.feedAt = "已记录，今日 1/8 杯", tt.Now()
	tt.Frame()
	settle(panelIdleHeight + panelFeedback)

	tt.Advance(FeedbackFlash + 500*time.Millisecond) // ...and it expires
	settle(panelIdleHeight)

	// Starting the timer opens the work card into the countdown.
	m.snap = core.Snapshot{Phase: core.PhaseRunning, Remaining: 25 * time.Minute, Total: 25 * time.Minute}
	tt.Frame()
	settle(panelRunHeight)

	// The panel reopens: the fresh window measures its content anew.
	p.ResetHeight()
	tt.Frame()
	settle(panelRunHeight)
}
