// Package app wires Restify's engines to its windows and the system:
// the tray and its panel, the settings and statistics windows, the
// reminder popup, the rest overlay, notifications, sounds and shortcuts.
package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"restify/internal/core"
	"restify/internal/gui"
	"restify/internal/store"
)

//go:embed trayicons/*.png
var trayIcons embed.FS

// Version is the app's version, shown in the panel and About page.
const Version = "1.0.0"

// App owns everything the running app needs.
type App struct {
	// Data
	set        *store.SettingsStore
	recs       *store.RecordsStore
	timerState *store.TimerStateStore

	// Engines
	Work      *core.WorkTimer
	Hydration *core.Hydration
	Lock      *core.LockSession

	// Views (their Model methods live on App)
	popover     gui.Popover
	settingsWin gui.SettingsWindow
	statsWin    gui.StatsWindow
	alertWin    gui.HydrationAlert
	lockView    gui.LockOverlay

	// UI state, guarded by mu.
	mu          sync.Mutex
	tray        *mygo.Tray
	flyout      *mygo.Window
	settings    *mygo.Window
	stats       *mygo.Window
	popup       *mygo.Window
	overlays    []*mygo.Window
	keepAwake   func() // releases the rest's wake hold, nil when none
	blinkOn     bool
	blinkStop   chan struct{}
	feedback    string
	feedAt      time.Time
	notice      string    // the last test notification's outcome, for the settings row
	noticeAt    time.Time // when that outcome was reported
	popupMsg    string    // the message the popup shows, cleared as it closes
	deniedUntil time.Time // the popup's record was denied until this moment
	snooze      *time.Timer

	// Sound state, guarded by soundMu: one event sound per kind, and the
	// settings rows' one audition. The players themselves are
	// goroutine-safe; the slots say which of them is current.
	soundMu sync.Mutex
	sounds  map[string]*soundVoice // reminder sounds by kind
	preview *soundVoice            // the audition playing or paused
}

// New loads the stores and builds the engines with their hooks.
func New(dir string) (*App, error) {
	set, err := store.OpenSettings(dir)
	if err != nil {
		return nil, err
	}
	recs, err := store.OpenRecords(dir)
	if err != nil {
		return nil, err
	}
	timerState, err := store.OpenTimerState(dir)
	if err != nil {
		return nil, err
	}

	a := &App{
		set:        set,
		recs:       recs,
		timerState: timerState,
	}
	a.popover = gui.Popover{Model: a}
	// The panel sizes its flyout to its content as the content settles.
	a.popover.OnHeight = func(h int) {
		a.mu.Lock()
		f := a.flyout
		a.mu.Unlock()
		if f != nil {
			f.Update(func() { f.SetContentSize(panelWidth, h) })
		}
	}
	a.settingsWin = gui.SettingsWindow{Model: a}
	a.statsWin = gui.StatsWindow{Model: a}
	a.alertWin = gui.HydrationAlert{Model: a}
	a.lockView = gui.LockOverlay{Model: a}

	a.Work = core.NewWorkTimer(
		core.WorkDeps{IdleTime: func() time.Duration { return mygo.Power.IdleTime() }},
		core.WorkHooks{
			Tick:   func(core.Snapshot) { a.tickUI() },
			Change: func(core.Snapshot) { a.tickUI() },
			Break:  func(core.Snapshot) { a.workBreak() },
			Persist: func(s core.Snapshot, start time.Time, running bool) {
				a.persistTimer(s, start, running)
			},
			IdlePause: func() { a.notifySimple("计时器已暂停", "检测到系统闲置，工作计时器已自动暂停") },
			SmartReminder: func() {
				a.notifySimple("检测到您已开始工作", "记得启动 Restify 工作计时器，保持良好的工作节奏哦！")
			},
			Blink: func(on bool) { a.setBlink(on) },
		})

	a.Hydration = core.NewHydration(recs, set, core.HydrationHooks{
		Remind: func() { a.hydrationRemind() },
		Celebrate: func(goal int) {
			a.playSound("success", 0.6)
			a.notifySimple("目标达成！", fmt.Sprintf("恭喜！今天已喝 %d 次水！", goal))
		},
		NewDay: func(yesterday int) {
			a.notifySimple("新的一天", fmt.Sprintf("昨天喝了 %d 次水，今天继续加油！", yesterday))
		},
		Denied: func(secs int) { a.setFeedback(fmt.Sprintf("请等待 %d 秒后再记录", secs)) },
		Change: func() { a.tickUI() },
	})

	a.Lock = core.NewLockSession(core.LockHooks{
		Tick: func(time.Duration) { a.updateOverlays() },
		Done: func() { a.endRest() },
	})

	return a, nil
}

// NewForTest builds the app over a throwaway directory: the engines and
// model methods work headlessly; Start (the tray, the windows) does not.
func NewForTest(dir string) (*App, error) { return New(dir) }

// PopoverView exposes the tray panel's view for tests.
func (a *App) PopoverView() func(c *ui.Context) { return a.popover.View }

// Start brings the app up, inside App.WhenReady.
func (a *App) Start() {
	s := a.set.Get()

	// The theme the settings name.
	a.applyThemeMode(s.ThemeMode)

	// The work timer comes back from what was persisted.
	saved := a.timerState.Get()
	a.Work.Restore(core.SnapshotPersisted{
		Start:     saved.StartTime,
		Running:   saved.Running,
		Remaining: saved.Remaining,
	}, s.WorkDuration)
	a.Work.SetSmartReminder(s.EnableSmartReminder, s.SmartReminderThreshold)

	// The hydration loop.
	a.Hydration.Start(time.Now())

	// The tray, the shortcuts, the login item.
	a.setupTray(s)
	a.registerShortcuts()
	if s.LaunchAtLogin {
		if err := mygo.App.SetOpenAtLogin(true); err != nil {
			log.Println("set open at login:", err)
		}
	}

	if s.AutoStartTimer {
		a.Work.Start(s.WorkDuration)
	}
}

// Stop saves what the engines hold and halts them (App.OnWillQuit).
func (a *App) Stop() {
	a.Hydration.Stop()
	a.releaseAwake()
	a.mu.Lock()
	snooze := a.snooze
	a.snooze = nil
	a.mu.Unlock()
	if snooze != nil {
		snooze.Stop()
	}
}

// SetSetting applies one settings change and its side effects.
func (a *App) SetSetting(fn func(*store.Settings)) {
	if err := a.set.Update(fn); err != nil {
		log.Println("save settings:", err)
	}
	a.settingsChanged()
}

// settingsChanged runs the side effects of any settings change.
func (a *App) settingsChanged() {
	s := a.set.Get()
	a.applyThemeMode(s.ThemeMode)
	a.Hydration.Reschedule()
	a.Work.SetSmartReminder(s.EnableSmartReminder, s.SmartReminderThreshold)
	a.refreshTray()
	a.tickUI()
}

func (a *App) applyThemeMode(m store.ThemeMode) {
	switch m {
	case store.ThemeLight:
		mygo.Theme.SetSource(mygo.ThemeLight)
	case store.ThemeDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// persistTimer keeps timer-state.json in step with the engine.
func (a *App) persistTimer(s core.Snapshot, start time.Time, running bool) {
	if !running && s.Remaining == 0 {
		if err := a.timerState.Clear(); err != nil {
			log.Println("clear timer state:", err)
		}
		return
	}
	if err := a.timerState.Save(store.TimerState{
		StartTime: start,
		Running:   running,
		Remaining: s.Remaining,
	}); err != nil {
		log.Println("save timer state:", err)
	}
}

// workBreak handles the end of a work period.
func (a *App) workBreak() {
	s := a.set.Get()
	if s.EnableAutoLock {
		a.Lock.Start(s.GracePeriod, time.Now())
		a.showOverlays()
		a.holdAwake()
		a.playSound("break", 1.0)
		return
	}
	a.playSound("break", 1.0)
	msg := core.PickBreakMessage(s.BreakMessageLibrary, s.EnableMotivationalMessages, s.WorkDuration)
	a.notifySimple("休息时间到了！", msg)
}

// holdAwake keeps the machine and its displays awake while the forced
// rest runs: the countdown stays up, and the machine is on when the user
// comes back to it.
func (a *App) holdAwake() {
	a.mu.Lock()
	held := a.keepAwake != nil
	a.mu.Unlock()
	if held {
		return
	}
	release := mygo.Power.KeepAwake("Restify 强制休息中", true)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.keepAwake != nil {
		release() // a concurrent hold won; drop ours
		return
	}
	a.keepAwake = release
}

// releaseAwake lets the machine idle-sleep on its own schedule again.
func (a *App) releaseAwake() {
	a.mu.Lock()
	release := a.keepAwake
	a.keepAwake = nil
	a.mu.Unlock()
	if release != nil {
		release()
	}
}

// hydrationRemind sends the drink reminder through every channel.
func (a *App) hydrationRemind() {
	s := a.set.Get()
	a.playSound("hydration", 0.8)
	// One message per reminder: the notification and the popup it may
	// open show the same words.
	msg := core.PickHydrationMessage(s.HydrationMessageLibrary)
	a.setPopupMessage(msg)
	body := core.HydrationBodyWithStats(msg, a.Hydration.TodayCount(time.Now()), s.DailyGoal)
	a.notify("hydration", "该喝水了！", body, func() { a.showHydrationPopup() })
	if s.EnablePopupAlert {
		a.showHydrationPopup()
	}
}

// setPopupMessage fixes the message the popup shows.
func (a *App) setPopupMessage(msg string) {
	a.mu.Lock()
	a.popupMsg = msg
	a.mu.Unlock()
}

// ---- UI plumbing ----

// tickUI refreshes the open windows and the tray after engine changes.
// The flyout's size follows its content on its own, through
// popover.OnHeight.
func (a *App) tickUI() {
	a.refreshTray()
	a.mu.Lock()
	wins := []*mygo.Window{a.flyout, a.settings, a.stats, a.popup}
	overlays := append([]*mygo.Window(nil), a.overlays...)
	a.mu.Unlock()
	for _, w := range wins {
		if w != nil {
			w.Update(func() {})
		}
	}
	for _, w := range overlays {
		w.Update(func() {})
	}
}

// setFeedback shows a toast in the panel for a moment.
func (a *App) setFeedback(text string) {
	a.mu.Lock()
	a.feedback, a.feedAt = text, time.Now()
	a.mu.Unlock()
	a.tickUI()
}

// ---- gui.PopoverModel ----

func (a *App) Timer() core.Snapshot { return a.Work.Snapshot() }

func (a *App) TodayCount() int { return a.Hydration.TodayCount(time.Now()) }

func (a *App) DailyGoal() int { return a.set.Get().DailyGoal }

func (a *App) RecordDrink() (bool, int) {
	ok, wait := a.Hydration.RecordDrink(time.Now())
	if ok {
		a.playSound("success", 0.6)
		a.setFeedback(fmt.Sprintf("已记录，今日 %d/%d 杯", a.TodayCount(), a.DailyGoal()))
	}
	return ok, wait
}

func (a *App) DrinkFeedback() (string, time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.feedback, a.feedAt
}

func (a *App) ToggleTimer() { a.Work.Toggle(a.set.Get().WorkDuration) }
func (a *App) ResetTimer() {
	a.Lock.Stop()
	a.endRest()
	a.Work.Reset()
}
func (a *App) SkipBreak()      { a.Work.SkipBreak(a.set.Get().WorkDuration) }
func (a *App) OpenSettings()   { a.showSettings() }
func (a *App) OpenStats()      { a.showStats() }
func (a *App) Version() string { return Version }

// ---- gui.SettingsModel ----

func (a *App) SettingsData() store.Settings { return a.set.Get() }

// Settings is named SettingsData above for the popover; the settings
// window's Settings() maps to it.
func (a *App) Settings() store.Settings { return a.set.Get() }

func (a *App) ImportSound(kind string) string {
	path, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:   "选择声音文件",
		Filters: []mygo.FileFilter{{Name: "声音", Extensions: []string{"mp3", "wav", "ogg"}}},
	})
	if err != nil || path == nil || len(path) == 0 {
		return ""
	}
	return path[0]
}

func (a *App) AddMessage(kind, text string) {
	a.SetSetting(func(s *store.Settings) {
		if kind == "break" {
			s.BreakMessageLibrary = append(s.BreakMessageLibrary, text)
		} else {
			s.HydrationMessageLibrary = append(s.HydrationMessageLibrary, text)
		}
	})
}

func (a *App) RemoveMessage(kind string, i int) {
	a.SetSetting(func(s *store.Settings) {
		var lib *[]string
		if kind == "break" {
			lib = &s.BreakMessageLibrary
		} else {
			lib = &s.HydrationMessageLibrary
		}
		if i >= 0 && i < len(*lib) {
			*lib = append((*lib)[:i], (*lib)[i+1:]...)
		}
	})
}

func (a *App) ResetMessages(kind string) {
	d := store.DefaultSettings()
	a.SetSetting(func(s *store.Settings) {
		if kind == "break" {
			s.BreakMessageLibrary = d.BreakMessageLibrary
		} else {
			s.HydrationMessageLibrary = d.HydrationMessageLibrary
		}
	})
}

// SendTestNotification shows a notification to check the permission, and
// reports how it went: the settings row displays the outcome instead of
// the click failing silently when this run cannot show notifications or
// the user has denied them.
func (a *App) SendTestNotification() {
	a.notifyResult("test", "测试通知", "Restify 的通知工作正常！", nil, func(err error) {
		a.setSettingsNotice(notificationResultNotice(err))
	})
}

// NotificationNotice is the outcome of the last test notification, and
// when it was reported (zero time when none).
func (a *App) NotificationNotice() (string, time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.notice, a.noticeAt
}

// setSettingsNotice fixes the settings row's notice and refreshes the
// windows it may show in.
func (a *App) setSettingsNotice(text string) {
	a.mu.Lock()
	a.notice, a.noticeAt = text, time.Now()
	a.mu.Unlock()
	a.tickUI()
}

func (a *App) ExportData() (string, error) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       "导出 Restify 数据",
		DefaultPath: filepath.Join(os.Getenv("HOME"), "Desktop", "restify-export.json"),
	})
	if err != nil || path == "" {
		return "", err
	}
	data := struct {
		Settings store.Settings `json:"settings"`
		Records  []store.Record `json:"hydrationRecords"`
		Exported time.Time      `json:"exportedAt"`
	}{a.set.Get(), a.recs.All(), time.Now()}
	blob, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) ClearAllData() bool {
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageWarning,
		Message: "清除全部数据？",
		Detail:  "设置与喝水记录都会删除，且无法恢复。",
		Buttons: []string{"清除", "取消"},
	})
	if err != nil || res.Button != 0 {
		return false
	}
	if err := a.recs.ClearAll(); err != nil {
		log.Println("clear records:", err)
	}
	a.SetSetting(func(s *store.Settings) { *s = store.DefaultSettings() })
	a.Hydration.Start(time.Now()) // reload today's count (zero)
	return true
}

func (a *App) ApplyLaunchAtLogin(on bool) {
	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		log.Println("set open at login:", err)
	}
}

// ---- gui.StatsModel ----

func (a *App) WeekStats() []store.DayCount  { return a.recs.WeekStats(time.Now()) }
func (a *App) MonthStats() []store.DayCount { return a.recs.MonthStats(time.Now()) }
func (a *App) ConsecutiveDays() int         { return a.recs.ConsecutiveDays(time.Now()) }
func (a *App) HighestRecord() int           { return a.recs.HighestRecord() }
func (a *App) AverageDaily() float64        { return a.recs.AverageDaily(time.Now()) }

func (a *App) Badges() (streak7 bool, goal30 bool, goalDays int, highest bool) {
	goal := a.DailyGoal()
	days, _ := a.recs.Badges(goal)
	return a.recs.ConsecutiveDays(time.Now()) >= 7, days >= 30, days, a.recs.HighestRecord() >= 8
}

// ---- gui.HydrationAlertModel ----

// HydrationMessage is the popup's message: the one the reminder chose, or
// a pick made once for a popup opened without one. It stays the same
// while the popup shows, whatever redraws come.
func (a *App) HydrationMessage() string {
	a.mu.Lock()
	msg := a.popupMsg
	a.mu.Unlock()
	if msg != "" {
		return msg
	}
	msg = core.PickHydrationMessage(a.set.Get().HydrationMessageLibrary)
	a.setPopupMessage(msg)
	return msg
}

// RecordDrinkAndClose records the drink and closes the popup when it took;
// a record too soon after the last keeps the popup up, showing the wait.
func (a *App) RecordDrinkAndClose() {
	ok, wait := a.RecordDrink()
	if !ok {
		a.mu.Lock()
		a.deniedUntil = time.Now().Add(time.Duration(wait) * time.Second)
		a.mu.Unlock()
		return // the denial's toast reached the panel; the popup says it too
	}
	a.closeHydrationPopup()
}

// DrinkDenial is the seconds the popup's last record still has to wait.
func (a *App) DrinkDenial() int {
	a.mu.Lock()
	until := a.deniedUntil
	a.mu.Unlock()
	secs := int((time.Until(until) + time.Second - 1) / time.Second)
	if secs < 0 {
		return 0
	}
	return secs
}

func (a *App) SnoozeAndClose() {
	a.closeHydrationPopup()
	a.mu.Lock()
	if a.snooze != nil {
		a.snooze.Stop()
	}
	// The nudge goes through the loop's own gate, so a goal met or a
	// reminder switched off in between stays quiet.
	a.snooze = time.AfterFunc(10*time.Minute, func() { a.Hydration.CheckAndRemind(time.Now()) })
	a.mu.Unlock()
	a.setFeedback("好的，10 分钟后再提醒你喝水")
}

// ---- gui.LockModel ----

func (a *App) RestRemaining() time.Duration { return a.Lock.Remaining() }
func (a *App) RestTotal() time.Duration     { return a.set.Get().GracePeriod }

// EndRest ends the rest overlay.
func (a *App) EndRest() { a.endRest() }

// endRest closes the overlay screens and stops the session. Whatever
// played for the rest ends with it: a custom break sound must not keep
// playing after the user forces the overlay away, or after the countdown
// runs out.
func (a *App) endRest() {
	a.Lock.Stop()
	a.closeOverlays()
	a.releaseAwake()
	a.stopRestSounds()
	a.tickUI()
}

// registerShortcuts wires the three global shortcuts.
func (a *App) registerShortcuts() {
	register := func(accel, name string, fn func()) {
		if err := mygo.GlobalShortcut.Register(accel, fn); err != nil {
			log.Printf("shortcut %s (%s): %v", name, accel, err)
		}
	}
	register("CmdOrCtrl+Shift+T", "toggle timer", func() {
		a.ToggleTimer()
		snap := a.Work.Snapshot()
		if snap.Phase == core.PhaseRunning {
			a.notifySimple("计时开始", "工作 "+core.FormatShort(snap.Total)+"，休息 "+core.FormatShort(a.set.Get().GracePeriod))
		}
	})
	register("CmdOrCtrl+Shift+H", "record drink", func() {
		if ok, wait := a.Hydration.RecordDrink(time.Now()); ok {
			a.playSound("success", 0.6)
			goal := a.DailyGoal()
			left := goal - a.TodayCount()
			if left > 0 {
				a.notifySimple("已记录喝水", fmt.Sprintf("今日已喝 %d 次，还差 %d 次", a.TodayCount(), left))
			} else {
				a.notifySimple("已记录喝水", fmt.Sprintf("今日已喝 %d 次，目标完成！", a.TodayCount()))
			}
		} else {
			a.notifySimple("记录太快啦", fmt.Sprintf("请等待 %d 秒后再记录", wait))
		}
	})
	register("CmdOrCtrl+,", "open settings", func() { a.showSettings() })
}

// trayIconPNG reads one generated template icon.
func trayIconPNG(style store.IconStyle, running bool) []byte {
	state := "idle"
	if running {
		state = "running"
	}
	data, err := trayIcons.ReadFile("trayicons/" + string(style) + "-" + state + ".png")
	if err != nil {
		log.Println("tray icon:", err)
		return nil
	}
	return data
}
