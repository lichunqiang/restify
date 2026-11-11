//go:build darwin

package app

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// AppKit's panel style and window level for a rest cover, as NSWindow's:
// a panel without the titled bit (NSWindowStyleMaskNonactivatingPanel
// alone, so AppKit no longer clamps its frame below the menu bar), and
// NSStatusWindowLevel, one above NSMainMenuWindowLevel (24) and above
// NSDockWindowLevel (20), so that the cover reaches over the menu bar and
// the Dock too.
const (
	panelStyleBorderless = 1 << 7 // NSWindowStyleMaskNonactivatingPanel
	statusWindowLevel    = 25     // NSStatusWindowLevel
)

var (
	coverSelectors sync.Once
	setStyleMask   objc.SEL
	setLevel       objc.SEL
)

// coverRestDisplay makes w cover the whole display r: a frameless panel
// is still a titled window to AppKit, which clamps such a window's frame
// to the display's work area when it shows — below the menu bar, and off
// the Dock — so the cover turns the window borderless and raises it over
// the menu bar before it shows. Both calls are public AppKit API, which
// MyGo does not pass on yet; a later MyGo that does could drop them.
func coverRestDisplay(w *mygo.Window, r mygo.Rectangle) {
	mygo.RunOnMain(func() {
		coverSelectors.Do(func() {
			setStyleMask = objc.RegisterName("setStyleMask:")
			setLevel = objc.RegisterName("setLevel:")
		})
		ns := objc.ID(w.NativeHandle())
		if ns == 0 {
			return
		}
		objc.Send[objc.ID](ns, setStyleMask, uint(panelStyleBorderless))
		objc.Send[objc.ID](ns, setLevel, statusWindowLevel)
	})
	w.SetBounds(r)
}
