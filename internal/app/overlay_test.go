package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// A rest cover spans the whole display, the menu bar and the Dock
// included, and never its work area: a cover laid out to the work area
// leaves a strip of the desktop above it, which is the not-full-screen
// bug this guards.
func TestRestCoverSpansWholeDisplay(t *testing.T) {
	displays := []mygo.Display{
		{
			Label: "primary", Primary: true,
			// The menu bar leaves a work area 33 DIPs down.
			Bounds:   mygo.Rectangle{X: 0, Y: 0, Width: 1512, Height: 982},
			WorkArea: mygo.Rectangle{X: 0, Y: 33, Width: 1512, Height: 949},
		},
		{
			Label: "secondary left of the primary",
			// The primary sits at (0,0) in the menu bar's DIPs, so another
			// display can be at a negative origin.
			Bounds:   mygo.Rectangle{X: -1920, Y: -240, Width: 1920, Height: 1080},
			WorkArea: mygo.Rectangle{X: -1920, Y: -240, Width: 1920, Height: 1040},
		},
		{
			Label:    "another",
			Bounds:   mygo.Rectangle{X: 1512, Y: 0, Width: 2560, Height: 1440},
			WorkArea: mygo.Rectangle{X: 1512, Y: 25, Width: 2560, Height: 1415},
		},
	}
	for _, d := range displays {
		if got := restCoverBounds(d); got != d.Bounds {
			t.Errorf("%s: cover bounds = %+v, want the display's bounds %+v", d.Label, got, d.Bounds)
		}
		if d.WorkArea != d.Bounds {
			if got := restCoverBounds(d); got == d.WorkArea {
				t.Errorf("%s: cover bounds = %+v, the work area, not the display", d.Label, got)
			}
		}

		o := restCoverOptions(d)
		if o.X != d.Bounds.X || o.Y != d.Bounds.Y || o.Width != d.Bounds.Width || o.Height != d.Bounds.Height {
			t.Errorf("%s: window at (%d,%d %dx%d), want (%d,%d %dx%d)", d.Label,
				o.X, o.Y, o.Width, o.Height, d.Bounds.X, d.Bounds.Y, d.Bounds.Width, d.Bounds.Height)
		}
		// A cover shows over the app the user is in, and until the app
		// shows it.
		if !o.Frameless || !o.Panel || !o.AlwaysOnTop || !o.Hidden {
			t.Errorf("%s: options %+v are not a hidden, always-on-top, frameless panel", d.Label, o)
		}
		if !o.DisableResize || !o.DisableMove || !o.DisableMinimize || !o.DisableMaximize || !o.DisableFullScreen {
			t.Errorf("%s: options %+v leave the cover resizable, movable, minimizable, maximizable or full-screenable", d.Label, o)
		}
	}
}
