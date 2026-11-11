package gui

import (
	"time"

	"restify/internal/core"
	"restify/internal/store"
)

// The views read the app through small interfaces, so they can render a
// fake in tests and the real app at runtime.

// PopoverModel is what the menu-bar panel shows and does.
type PopoverModel interface {
	Timer() core.Snapshot
	Settings() store.Settings
	TodayCount() int
	DailyGoal() int
	// RecordDrink logs a drink; it reports a denial and the seconds to
	// wait.
	RecordDrink() (ok bool, waitSecs int)
	// DrinkFeedback is the toast text after the last record attempt, and
	// when it was shown (zero when none).
	DrinkFeedback() (text string, at time.Time)
	ToggleTimer()
	ResetTimer()
	SkipBreak()
	OpenSettings()
	OpenStats()
	Version() string
}

// SettingsModel edits the settings and reads what the pages show.
type SettingsModel interface {
	Settings() store.Settings
	SetSetting(func(*store.Settings)) // applies and persists one change
	TodayCount() int
	// ImportSound asks for a sound file for kind ("break" or "hydration")
	// and returns its path, empty when canceled.
	ImportSound(kind string) string
	// TestSound auditions kind's reminder sound: a click starts it, pauses
	// it while it plays, resumes it while it waits paused.
	TestSound(kind string)
	// SoundPreviewState reports whether kind's audition plays or waits
	// paused, so the row's button can say what a click does.
	SoundPreviewState(kind string) (playing, paused bool)
	AddMessage(kind string, text string)
	RemoveMessage(kind string, i int)
	ResetMessages(kind string) // "break" or "hydration"
	SendTestNotification()
	// NotificationNotice is the outcome of the last test notification and
	// when it was reported (zero time when none). The row shows it in
	// place of its hint for a while.
	NotificationNotice() (text string, at time.Time)
	ExportData() (string, error) // writes a JSON export, returns its path
	ClearAllData() bool          // asks, clears on yes
	ApplyLaunchAtLogin(bool)
	Version() string
}

// StatsModel feeds the statistics window.
type StatsModel interface {
	TodayCount() int
	DailyGoal() int
	WeekStats() []store.DayCount
	MonthStats() []store.DayCount
	ConsecutiveDays() int
	HighestRecord() int
	AverageDaily() float64
	// BadgeReached reports whether each achievement is earned.
	Badges() (streak7 bool, goalDays30 bool, goalDays int, highest bool)
}

// HydrationAlertModel is the drink reminder popup.
type HydrationAlertModel interface {
	// HydrationMessage is the one message this popup shows, chosen when
	// the popup came up.
	HydrationMessage() string
	TodayCount() int
	DailyGoal() int
	RecordDrinkAndClose()
	// DrinkDenial is the seconds to wait after the popup's record was
	// denied as too soon, 0 when the last record went through (or none
	// was attempted).
	DrinkDenial() int
	SnoozeAndClose()
}

// LockModel is the forced-rest overlay.
type LockModel interface {
	RestRemaining() time.Duration
	RestTotal() time.Duration
	EndRest() // "force exit" during, "done resting" after the countdown
}

// FeedbackFlash is how long a panel toast shows (Swift showed 3s).
const FeedbackFlash = 3 * time.Second

// NoticeFlash is how long the settings row shows a test notification's
// outcome: its guidance (open System Settings) needs reading, so longer
// than a toast.
const NoticeFlash = 10 * time.Second
