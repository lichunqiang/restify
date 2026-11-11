package app

import (
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"restify/internal/core"
	"restify/internal/store"
)

// setupTray creates the menu-bar item: a template icon naming the timer
// state, the countdown as its title, the panel on click and a menu on
// right-click.
func (a *App) setupTray(s store.Settings) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "restify-main",
		Icon:           trayIconPNG(s.IconStyle, false),
		IconIsTemplate: true,
		ToolTip:        "Restify — 休息与喝水提醒",
	})
	if err != nil {
		log.Println("tray:", err)
		return
	}
	a.mu.Lock()
	a.tray = tray
	a.mu.Unlock()

	tray.OnClick(func() {
		a.stopBlink()
		a.toggleFlyout()
	})
	// Right-click (and Control-click) shows the menu, none on plain
	// clicks, which open the panel.
	tray.OnRightClick(func() {
		tray.SetMenu(a.trayMenu())
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})

	a.refreshTray()
}

// trayMenu builds the right-click menu. The update item names a found
// version when there is one: the menu rebuilds on every right-click, so
// the label follows the mirror.
func (a *App) trayMenu() *mygo.Menu {
	update := "检查更新…"
	if v := a.UpdateAvailable(); v != "" {
		update = "发现新版本 " + v + "…"
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.showSettings() }},
		{Label: "饮水统计…", Click: func(*mygo.MenuItem, *mygo.Window) { a.showStats() }},
		{Label: update, Click: func(*mygo.MenuItem, *mygo.Window) { a.CheckForUpdates() }},
		mygo.Separator(),
		{Label: "跳过休息", Click: func(*mygo.MenuItem, *mygo.Window) { a.SkipBreak() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
}

// refreshTray updates the icon and title for the timer's state.
func (a *App) refreshTray() {
	a.mu.Lock()
	tray, blinkOn := a.tray, a.blinkOn
	a.mu.Unlock()
	if tray == nil {
		return
	}
	s := a.set.Get()
	snap := a.Work.Snapshot()

	running := snap.Phase == core.PhaseRunning
	if blinkOn {
		// Blinking swaps the glyph each beat; the running icon stands in
		// for the lit beat.
		running = !running
	}
	icon := trayIconPNG(s.IconStyle, running)
	if icon != nil && s.ShowMenuBarIcon {
		tray.SetIcon(icon, true)
	}

	title := ""
	if snap.Phase != core.PhaseIdle {
		title = padTitle(core.FormatClock(snap.Remaining))
	}
	tray.SetTitle(title)
}

// setBlink turns the nudge blink on or off.
func (a *App) setBlink(on bool) {
	a.mu.Lock()
	was := a.blinkOn
	stop := a.blinkStop
	a.blinkOn = on
	a.blinkStop = nil
	a.mu.Unlock()
	if was == on {
		return
	}
	if stop != nil {
		close(stop)
	}
	if on {
		stop = make(chan struct{})
		a.mu.Lock()
		a.blinkStop = stop
		a.mu.Unlock()
		go a.blinkLoop(stop)
	}
	a.refreshTray()
}

// stopBlink ends the blink from a user interaction.
func (a *App) stopBlink() { a.setBlink(false) }

func (a *App) blinkLoop(stop chan struct{}) {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	var flip atomic.Bool
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			flip.Store(!flip.Load())
			a.mu.Lock()
			blinking := a.blinkOn
			a.mu.Unlock()
			if !blinking {
				return
			}
			// Refresh with the flipped glyph.
			a.mu.Lock()
			tray := a.tray
			a.mu.Unlock()
			if tray == nil {
				return
			}
			s := a.set.Get()
			running := flip.Load()
			if icon := trayIconPNG(s.IconStyle, running); icon != nil {
				tray.SetIcon(icon, true)
			}
		}
	}
}

// padTitle keeps the countdown's width steady with figure spaces
// (U+2007), as wide as a digit, so the menu bar's items don't shift.
func padTitle(s string) string {
	const width = 5
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(r)) + s
}
