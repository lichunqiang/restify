// Restify keeps work healthy: a work timer with a forced rest, and
// drink reminders — in the menu bar, in Go.
package main

import (
	"log"

	"github.com/egoist/mygo"

	"restify/internal/app"
)

func main() {
	// Dev runs are named after the binary; tie them to the app so the
	// data directory and the single-instance lock are stable.
	mygo.App.SetName("Restify")
	mygo.App.SetVersion(app.Version)

	// A menu-bar app: no Dock icon.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	// The tray keeps the app alive with no window.
	mygo.App.OnWindowAllClosed(func() {})

	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	a, err := app.New(dir)
	if err != nil {
		log.Fatal(err)
	}

	// A click on a Notification Center reminder opens the popup, also
	// when the click launched the app anew.
	mygo.App.OnNotificationClick(func(id string) {
		if id == "hydration" {
			a.OpenHydrationPopup()
		}
	})
	// What the user has seen in the app doesn't wait in Notification
	// Center.
	mygo.App.OnDidBecomeActive(func() { mygo.ClearNotifications() })

	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.Stop() })

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnSecondInstance(func([]string, string) {
		a.OpenSettings()
	})

	mygo.App.WhenReady(func() { a.Start() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
