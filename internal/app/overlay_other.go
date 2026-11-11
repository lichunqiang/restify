//go:build !darwin

package app

import "github.com/egoist/mygo"

// coverRestDisplay gives the rest cover the display's whole bounds. Other
// platforms place their windows differently, and this app is a macOS menu
// bar app; the cover's bounds are all there is to set elsewhere.
func coverRestDisplay(w *mygo.Window, r mygo.Rectangle) { w.SetBounds(r) }
