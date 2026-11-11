package app

import (
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"restify/internal/gui"
)

// readFile is os.ReadFile, split for the sound cache's tests.
var readFile = os.ReadFile

// ---- Tray panel (flyout) ----

// panelWidth is the tray panel's width.
const panelWidth = 340

// panelFeedbackOn reports whether a drink toast is showing in the panel.
func (a *App) panelFeedbackOn() bool {
	text, at := a.DrinkFeedback()
	return text != "" && time.Since(at) < gui.FeedbackFlash
}

// toggleFlyout opens or closes the menu-bar panel.
func (a *App) toggleFlyout() {
	a.mu.Lock()
	open := a.flyout
	a.mu.Unlock()
	if open != nil {
		open.Close()
		return
	}
	a.mu.Lock()
	tray := a.tray
	flyoutPhase := a.Work.Snapshot().Phase
	a.mu.Unlock()

	a.popover.ResetHeight() // a fresh window measures its content anew
	f := mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      tray,
		Placement: mygo.PlacementBottom,
		Width:     panelWidth,
		Height:    gui.PanelHeight(flyoutPhase, a.panelFeedbackOn()),
		Focusable: true,
		Popover:   true,
		Content:   ui.View(a.popover.View),
	})
	a.mu.Lock()
	a.flyout = f
	a.mu.Unlock()
	f.OnClosed(func() {
		a.mu.Lock()
		a.flyout = nil
		a.mu.Unlock()
	})
}

// ---- Settings window ----

// showSettings opens (or reopens) the settings window.
func (a *App) showSettings() {
	a.mu.Lock()
	win := a.settings
	a.mu.Unlock()
	if win != nil {
		win.Show()
		win.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     "Restify 设置",
		Width:     720,
		Height:    560,
		MinWidth:  600,
		MinHeight: 480,
		StateKey:  "settings",
		Content:   ui.View(a.settingsWin.View),
	})
	a.mu.Lock()
	a.settings = w
	a.mu.Unlock()
	w.OnClosed(func() {
		a.mu.Lock()
		a.settings = nil
		a.mu.Unlock()
	})
}

// ---- Statistics window ----

// showStats opens (or reopens) the statistics window.
func (a *App) showStats() {
	a.mu.Lock()
	win := a.stats
	a.mu.Unlock()
	if win != nil {
		win.Show()
		win.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     "饮水统计",
		Width:     680,
		Height:    640,
		MinWidth:  560,
		MinHeight: 480,
		StateKey:  "stats",
		Content:   ui.View(a.statsWin.View),
	})
	a.mu.Lock()
	a.stats = w
	a.mu.Unlock()
	w.OnClosed(func() {
		a.mu.Lock()
		a.stats = nil
		a.mu.Unlock()
	})
}

// ---- Hydration popup ----

// showHydrationPopup brings up the drink reminder window; a reminder
// while it is up re-fronts it instead of stacking.
func (a *App) showHydrationPopup() {
	a.mu.Lock()
	popup := a.popup
	a.mu.Unlock()
	if popup != nil {
		popup.Show()
		popup.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:           "喝水提醒",
		Width:           380,
		Height:          460,
		AlwaysOnTop:     true,
		DisableResize:   true,
		DisableMaximize: true,
		DisableMinimize: true,
		SkipTaskbar:     true,
		Content:         ui.View(a.alertWin.View),
	})
	w.Center()
	a.mu.Lock()
	a.popup = w
	a.mu.Unlock()
	w.OnClosed(func() {
		a.mu.Lock()
		a.popup = nil
		a.popupMsg = "" // the next popup picks its message anew
		a.mu.Unlock()
	})
	w.Show()
	w.Focus()
}

// closeHydrationPopup dismisses the popup, if it is up.
func (a *App) closeHydrationPopup() {
	a.mu.Lock()
	popup := a.popup
	a.popup = nil
	a.mu.Unlock()
	if popup != nil {
		popup.Destroy()
	}
}

// ---- Rest overlay ----

// showOverlays covers every screen with the rest view, one window each.
func (a *App) showOverlays() {
	a.closeOverlays()
	var wins []*mygo.Window
	for _, d := range mygo.Screen.Displays() {
		o := restCoverOptions(d)
		o.Content = ui.View(a.lockView.View)
		w := mygo.NewWindow(o)
		// The cover spans the whole display, the menu bar and the Dock
		// included: the window is hidden here (mygo centers it, as the
		// primary display's X and Y are zeroes, and AppKit clamps a titled
		// window to the work area as it shows), so it can be given that
		// frame and the escape from the clamp before it shows
		// (coverRestDisplay).
		coverRestDisplay(w, restCoverBounds(d))
		w.SetVisibleOnAllWorkspaces(true)
		w.Show()
		wins = append(wins, w)
	}
	a.mu.Lock()
	a.overlays = wins
	a.mu.Unlock()
}

// updateOverlays redraws the overlay countdowns.
func (a *App) updateOverlays() { a.tickUI() }

// closeOverlays removes the rest covers.
func (a *App) closeOverlays() {
	a.mu.Lock()
	wins := a.overlays
	a.overlays = nil
	a.mu.Unlock()
	for _, w := range wins {
		w.Destroy()
	}
}

// OpenHydrationPopup is the notification-click entry to the popup.
func (a *App) OpenHydrationPopup() { a.showHydrationPopup() }
