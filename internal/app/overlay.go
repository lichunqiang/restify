package app

import (
	"github.com/egoist/mygo"
)

// restCoverOptions builds one display's rest cover: a frameless panel that
// fills its display, hidden until showOverlays shows it. The cover takes
// the Content of the app's LockOverlay view in showOverlays.
func restCoverOptions(d mygo.Display) mygo.WindowOptions {
	b := restCoverBounds(d)
	return mygo.WindowOptions{
		Frameless:         true,
		Panel:             true,
		AlwaysOnTop:       true,
		X:                 b.X,
		Y:                 b.Y,
		Width:             b.Width,
		Height:            b.Height,
		Hidden:            true,
		DisableResize:     true,
		DisableMove:       true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		DisableShadow:     true,
		SkipTaskbar:       true,
		BackgroundColor:   "#111114",
	}
}

// restCoverBounds is the rectangle a rest cover spans on display d: the
// whole display, the menu bar and the Dock included, and not its work
// area. A cover that stopped at the work area would leave a strip of the
// desktop above it, which is not a screen-covering rest.
func restCoverBounds(d mygo.Display) mygo.Rectangle { return d.Bounds }
