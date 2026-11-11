package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"restify/internal/gui"
)

// Updates come from the signed GitHub releases through mygo's updater
// plugin (registered in main): it owns the update window, the background
// checks and the preferences (updater.json). The app mirrors the state
// the views show, so they read the mirror instead of the plugin — the
// plugin's functions panic before mygo.Use, which headless tests must
// not hit.

// initUpdates seeds the mirror and follows the plugin's changes. It runs
// in Start, inside WhenReady; OnChange calls back on the main thread,
// like the engines' hooks.
func (a *App) initUpdates() {
	refresh := func() {
		a.mu.Lock()
		a.updAuto = updater.AutomaticChecks()
		a.updLastCheck = updater.LastCheck()
		a.updAvailable = updater.AvailableVersion()
		a.mu.Unlock()
	}
	refresh()
	updater.OnChange(func() {
		refresh()
		a.tickUI()
	})
}

// ---- gui.PopoverModel updates ----

func (a *App) UpdateAvailable() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updAvailable
}

// CheckForUpdates opens the update window now, as a user's click: it
// shows at once and says when the app is up to date, even when the build
// cannot update itself.
func (a *App) CheckForUpdates() { updater.CheckForUpdates() }

// ---- gui.SettingsModel updates ----

func (a *App) UpdateInfo() gui.UpdateInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return gui.UpdateInfo{
		AutoCheck: a.updAuto,
		LastCheck: a.updLastCheck,
		Available: a.updAvailable,
		CanUpdate: mygo.Updater.Enabled(),
	}
}

// SetUpdateAutoCheck turns the background checks on or off; the choice
// persists in the plugin's updater.json and comes back through OnChange.
func (a *App) SetUpdateAutoCheck(on bool) { updater.SetAutomaticChecks(on) }
