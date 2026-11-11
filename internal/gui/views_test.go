package gui

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"restify/internal/core"
	"restify/internal/store"
)

// uiNewTester renders a view headlessly at the tray panel's settings.
func uiNewTester(view func(c *ui.Context), w, h int) *ui.Tester {
	return ui.NewTester(view, w, h)
}

// fakePopoverModel records the panel's actions.
type fakePopoverModel struct {
	snap     core.Snapshot
	today    int
	goal     int
	recordOK bool
	waitSecs int
	feedback string
	feedAt   time.Time
	toggles  int
	resets   int
	skips    int
	settings int
	stats    int
	records  int
	avail    string
	checks   int
}

func (f *fakePopoverModel) Timer() core.Snapshot               { return f.snap }
func (f *fakePopoverModel) Settings() store.Settings           { return store.DefaultSettings() }
func (f *fakePopoverModel) TodayCount() int                    { return f.today }
func (f *fakePopoverModel) DailyGoal() int                     { return f.goal }
func (f *fakePopoverModel) RecordDrink() (bool, int)           { f.records++; return f.recordOK, f.waitSecs }
func (f *fakePopoverModel) DrinkFeedback() (string, time.Time) { return f.feedback, f.feedAt }
func (f *fakePopoverModel) ToggleTimer()                       { f.toggles++ }
func (f *fakePopoverModel) ResetTimer()                        { f.resets++ }
func (f *fakePopoverModel) SkipBreak()                         { f.skips++ }
func (f *fakePopoverModel) OpenSettings()                      { f.settings++ }
func (f *fakePopoverModel) OpenStats()                         { f.stats++ }
func (f *fakePopoverModel) Version() string                    { return "1.0.0" }
func (f *fakePopoverModel) UpdateAvailable() string            { return f.avail }
func (f *fakePopoverModel) CheckForUpdates()                   { f.checks++ }

func TestPopoverView(t *testing.T) {
	m := &fakePopoverModel{
		snap:     core.Snapshot{Phase: core.PhaseRunning, Remaining: 90 * time.Second, Total: 25 * time.Minute},
		today:    3,
		goal:     8,
		recordOK: true,
	}
	p := &Popover{Model: m}
	tt := uiNewTester(p.View, 340, 520)
	tt.SetVibrancy(true)

	for _, want := range []string{"Restify", "工作计时", "运行中", "01:30", "喝水提醒", "记录喝水", "38%"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing; texts %q", want, tt.Texts())
		}
	}

	if err := tt.Click("记录喝水"); err != nil {
		t.Fatal(err)
	}
	if m.records != 1 {
		t.Errorf("records = %d after click", m.records)
	}
	if err := tt.Click("暂停"); err != nil { // running shows the pause toggle
		t.Fatal(err)
	}
	if m.toggles != 1 {
		t.Errorf("toggles = %d after pause", m.toggles)
	}
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	if m.resets != 1 {
		t.Errorf("resets = %d after reset", m.resets)
	}
	if err := tt.Click("跳过休息"); err != nil {
		t.Fatal(err)
	}
	if m.skips != 1 {
		t.Errorf("skips = %d after skip", m.skips)
	}
	if err := tt.Click("设置"); err != nil { // the action icon names itself
		t.Fatal(err)
	}
	if m.settings != 1 {
		t.Errorf("settings opened %d times", m.settings)
	}
	if err := tt.Click("统计"); err != nil {
		t.Fatal(err)
	}
	if m.stats != 1 {
		t.Errorf("stats opened %d times", m.stats)
	}
}

// The ring's percent is a whole number, not Lobe's raw float (3/8 would
// read "37.5%" and overflow the circle).
func TestPopoverPercentIsWhole(t *testing.T) {
	m := &fakePopoverModel{today: 5, goal: 12, snap: core.Snapshot{Phase: core.PhaseIdle}}
	tt := uiNewTester((&Popover{Model: m}).View, 340, 520)
	if !tt.HasText("42%") {
		t.Errorf("whole percent missing; texts %q", tt.Texts())
	}
	for _, texts := range tt.Texts() {
		if len(texts) > 4 && texts[len(texts)-1] == '%' && texts[len(texts)-3] == '.' {
			t.Errorf("percent %q has decimals", texts)
		}
	}
}

func TestPopoverGoalReached(t *testing.T) {
	m := &fakePopoverModel{today: 8, goal: 8, snap: core.Snapshot{Phase: core.PhaseIdle}}
	tt := uiNewTester((&Popover{Model: m}).View, 340, 520)
	if !tt.HasText("已达标") {
		t.Errorf("goal tag missing; texts %q", tt.Texts())
	}
	if !tt.HasText("100%") {
		t.Errorf("ring percent missing; texts %q", tt.Texts())
	}
}

// Idle is compact: no countdown number, no running controls, and the start
// button is the only action of the work card.
func TestPopoverIdleCompact(t *testing.T) {
	m := &fakePopoverModel{snap: core.Snapshot{Phase: core.PhaseIdle}}
	tt := uiNewTester((&Popover{Model: m}).View, 340, 520)
	for _, want := range []string{"未开始", "开始工作", "工作 25分钟"} {
		if !tt.HasText(want) {
			t.Errorf("idle: %q missing; texts %q", want, tt.Texts())
		}
	}
	for _, gone := range []string{"00:00", "重置", "跳过休息"} {
		if tt.HasText(gone) {
			t.Errorf("idle: %q should be gone; texts %q", gone, tt.Texts())
		}
	}
	if err := tt.Click("开始工作"); err != nil {
		t.Fatal(err)
	}
	if m.toggles != 1 {
		t.Errorf("toggles = %d after start", m.toggles)
	}
}

func TestPopoverDeniedFeedback(t *testing.T) {
	m := &fakePopoverModel{
		snap:     core.Snapshot{Phase: core.PhaseIdle},
		feedback: "请等待 20 秒后再记录",
	}
	tt := uiNewTester((&Popover{Model: m}).View, 340, 520)
	m.feedAt = tt.Now()
	tt.Frame()
	if !tt.HasText("请等待 20 秒后再记录") {
		t.Errorf("feedback missing; texts %q", tt.Texts())
	}
}

// A found version takes the version string's place in the footer, as a
// link that opens the update window; with none, the version text stays.
func TestPopoverUpdateAvailable(t *testing.T) {
	m := &fakePopoverModel{snap: core.Snapshot{Phase: core.PhaseIdle}, avail: "1.1.0"}
	tt := uiNewTester((&Popover{Model: m}).View, 340, 520)
	if !tt.HasText("新版本 1.1.0") {
		t.Errorf("update hint missing; texts %q", tt.Texts())
	}
	if tt.HasText("Restify 1.0.0") {
		t.Errorf("version text shows over the update hint; texts %q", tt.Texts())
	}
	if err := tt.Click("新版本 1.1.0"); err != nil {
		t.Fatal(err)
	}
	if m.checks != 1 {
		t.Errorf("checks = %d after click", m.checks)
	}

	m.avail = ""
	tt.Frame()
	if !tt.HasText("Restify 1.0.0") {
		t.Errorf("version text missing after the update went away; texts %q", tt.Texts())
	}
	if tt.HasText("新版本 1.1.0") {
		t.Errorf("update hint stayed after the update went away; texts %q", tt.Texts())
	}
}

// The panel draws with the Lobe tokens, so both appearances render and
// they really differ: the same view in the dark appearance paints other
// colors than in the light one.
func TestPopoverAppearances(t *testing.T) {
	h := PanelHeight(core.PhaseRunning, false)
	render := func(dark bool) (*image.RGBA, []string) {
		m := &fakePopoverModel{snap: core.Snapshot{Phase: core.PhaseRunning, Remaining: time.Minute, Total: 25 * time.Minute}, today: 3, goal: 8}
		tt := uiNewTester((&Popover{Model: m}).View, 340, h)
		tt.SetDark(dark)
		tt.Advance(700 * time.Millisecond)
		tt.Frame()
		return tt.Image(), tt.Texts()
	}
	light, lightTexts := render(false)
	dark, darkTexts := render(true)
	if len(lightTexts) == 0 || len(darkTexts) == 0 {
		t.Fatalf("nothing rendered: light %d texts, dark %d", len(lightTexts), len(darkTexts))
	}
	if !hasText(darkTexts, "工作计时") || !hasText(darkTexts, "记录喝水") {
		t.Errorf("dark apperance lost the panel's labels: %q", darkTexts)
	}
	if !differs(light, dark) {
		t.Error("the panel paints the same colors in the light and the dark appearance: the Lobe tokens are not applied")
	}
}

// hasText reports whether texts holds want.
func hasText(texts []string, want string) bool {
	for _, s := range texts {
		if s == want {
			return true
		}
	}
	return false
}

// differs reports whether two images of one size paint different pixels.
func differs(a, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return true
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 3 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 3 {
			if a.At(x, y) != b.At(x, y) {
				return true
			}
		}
	}
	return false
}

// fakeSettingsModel records setting edits.
type fakeSettingsModel struct {
	set     store.Settings
	today   int
	edits   int
	notify  int
	exports int
	clears  int
	logins  int
	adds    []string
	removes int
	resets  int
	imports []string
	sounds  []string
	// soundPlaying/soundPaused stand in for the app's audition state, by
	// kind.
	soundPlaying map[string]bool
	soundPaused  map[string]bool
	exportErr    error
	stats        int
	notice       string
	noticeAt     time.Time
	updInfo      UpdateInfo
	checks       int
	autoChecks   int
	autoSet      bool
}

func (f *fakeSettingsModel) Settings() store.Settings { return f.set }
func (f *fakeSettingsModel) SetSetting(fn func(*store.Settings)) {
	f.edits++
	fn(&f.set)
}
func (f *fakeSettingsModel) TodayCount() int { return f.today }
func (f *fakeSettingsModel) ImportSound(kind string) string {
	f.imports = append(f.imports, kind)
	return "/tmp/sound.mp3"
}
func (f *fakeSettingsModel) TestSound(kind string) { f.sounds = append(f.sounds, kind) }
func (f *fakeSettingsModel) SoundPreviewState(kind string) (bool, bool) {
	return f.soundPlaying[kind], f.soundPaused[kind]
}
func (f *fakeSettingsModel) AddMessage(k, text string) { f.adds = append(f.adds, k+":"+text) }
func (f *fakeSettingsModel) RemoveMessage(string, int) { f.removes++ }
func (f *fakeSettingsModel) ResetMessages(string)      { f.resets++ }
func (f *fakeSettingsModel) SendTestNotification()     { f.notify++ }
func (f *fakeSettingsModel) NotificationNotice() (string, time.Time) {
	return f.notice, f.noticeAt
}
func (f *fakeSettingsModel) ClearAllData() bool      { f.clears++; return true }
func (f *fakeSettingsModel) ApplyLaunchAtLogin(bool) { f.logins++ }
func (f *fakeSettingsModel) Version() string         { return "1.0.0" }
func (f *fakeSettingsModel) OpenStats()              { f.stats++ }
func (f *fakeSettingsModel) ExportData() (string, error) {
	f.exports++
	return "/tmp/export.json", f.exportErr
}
func (f *fakeSettingsModel) UpdateInfo() UpdateInfo     { return f.updInfo }
func (f *fakeSettingsModel) CheckForUpdates()           { f.checks++ }
func (f *fakeSettingsModel) SetUpdateAutoCheck(on bool) { f.autoChecks++; f.autoSet = on }

func TestSettingsViewPages(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 560)

	for _, want := range []string{"工作", "喝水", "通用", "关于", "工作时长", "休息时长", "智能工作提醒"} {
		if !tt.HasText(want) {
			t.Errorf("work page: %q missing; texts %q", want, tt.Texts())
		}
	}

	// Navigate to 喝水.
	if err := tt.Click("喝水"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"每日目标", "目标杯数", "提醒间隔", "今日饮水"} {
		if !tt.HasText(want) {
			t.Errorf("hydration page: %q missing; texts %q", want, tt.Texts())
		}
	}

	// Navigate to 通用.
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"登录时启动", "提醒声音", "自定义声音", "提醒文案", "外观", "导出数据"} {
		if !tt.HasText(want) {
			t.Errorf("general page: %q missing; texts %q", want, tt.Texts())
		}
	}

	// Navigate to 关于.
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("版本") || !tt.HasText("1.0.0") {
		t.Errorf("about page missing version; texts %q", tt.Texts())
	}
}

func TestSettingsPresetEditsWorkDuration(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 560)
	if err := tt.Click("45 分钟"); err != nil {
		t.Fatal(err)
	}
	if m.set.WorkDuration != 45*time.Minute {
		t.Errorf("workDuration = %v after preset", m.set.WorkDuration)
	}
}

func TestSettingsTestNotification(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 560)
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("发送测试通知"); err != nil {
		t.Fatal(err)
	}
	if m.notify != 1 {
		t.Errorf("test notifications = %d", m.notify)
	}
}

// The audition button labels what a click does: 试听 while silent, 暂停
// while the sound plays, 继续 while it waits paused.
func TestSettingsSoundAuditionLabels(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	// Tall enough to show the whole general page, 自定义声音 included.
	tt := uiNewTester(s.View, 720, 1400)
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("试听") {
		t.Fatalf("idle audition button missing; texts %q", tt.Texts())
	}

	m.soundPlaying = map[string]bool{"break": true}
	m.soundPaused = map[string]bool{"hydration": true}
	tt.Frame()
	if !tt.HasText("暂停") {
		t.Errorf("playing audition button does not say 暂停; texts %q", tt.Texts())
	}
	if !tt.HasText("继续") {
		t.Errorf("paused audition button does not say 继续; texts %q", tt.Texts())
	}
}

// The test-notification row reports the outcome in place of its hint for
// a while, then the hint returns: a click never fails silently.
func TestSettingsNotificationNotice(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 560)
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("通知被拒绝") {
		t.Errorf("outcome without a test; texts %q", tt.Texts())
	}

	m.notice = "通知被拒绝：请在 系统设置 → 通知 → Restify 中允许通知"
	m.noticeAt = tt.Now()
	tt.Frame()
	if !tt.HasText(m.notice) {
		t.Errorf("outcome missing; texts %q", tt.Texts())
	}
	if tt.HasText("发送一条测试通知检查权限") {
		t.Errorf("hint shows over the outcome; texts %q", tt.Texts())
	}

	tt.Advance(NoticeFlash)
	tt.Frame()
	if !tt.HasText("发送一条测试通知检查权限") {
		t.Errorf("hint did not return after the flash; texts %q", tt.Texts())
	}
}

// The theme picker writes its choice back as a setting.
func TestSettingsThemeSegmented(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	// Tall enough to show the appearance card without scrolling.
	tt := uiNewTester(s.View, 720, 1400)
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	if m.set.ThemeMode != store.ThemeDark {
		t.Errorf("theme mode = %v after choosing 深色", m.set.ThemeMode)
	}
}

// The reminder interval is a select: choosing an option writes it back.
func TestSettingsIntervalSelect(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 560)
	if err := tt.Click("喝水"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("1 小时"); err != nil { // the trigger shows the current choice
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("30 分钟"); err != nil {
		t.Fatal(err)
	}
	if m.set.ReminderInterval != 30*time.Minute {
		t.Errorf("reminder interval = %v after choosing 30 分钟", m.set.ReminderInterval)
	}
}

// The About page's update card shows the checker's state, and a click on
// 立即检查 opens the update window.
func TestSettingsAboutUpdateCard(t *testing.T) {
	m := &fakeSettingsModel{
		set:     store.DefaultSettings(),
		updInfo: UpdateInfo{AutoCheck: true, CanUpdate: true},
	}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 720)
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"更新", "自动检查更新", "检查更新", "从 GitHub Release 下载并安装新版本"} {
		if !tt.HasText(want) {
			t.Errorf("about page: %q missing; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	if m.checks != 1 {
		t.Errorf("checks = %d after click", m.checks)
	}
}

// A found version says itself in the row's hint and the notice, and the
// row's button turns into 查看更新.
func TestSettingsAboutUpdateAvailable(t *testing.T) {
	m := &fakeSettingsModel{
		set:     store.DefaultSettings(),
		updInfo: UpdateInfo{AutoCheck: true, Available: "1.1.0", CanUpdate: true},
	}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 720)
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"发现新版本 1.1.0", "查看更新", "点击“查看更新”即可安装"} {
		if !tt.HasText(want) {
			t.Errorf("about page: %q missing; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("查看更新"); err != nil {
		t.Fatal(err)
	}
	if m.checks != 1 {
		t.Errorf("checks = %d after click", m.checks)
	}
}

// switchRect is a named switch's bounds: the row's label text shares the
// switch's name, so a click has to aim at the control itself.
func switchRect(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	info, ok := tt.AccessByRole(ui.RoleSwitch, label)
	if !ok {
		t.Fatalf("no switch named %s", label)
	}
	return info.Bounds
}

// The automatic-check switch writes through to the updater's preference.
func TestSettingsAboutUpdateSwitch(t *testing.T) {
	m := &fakeSettingsModel{set: store.DefaultSettings()}
	s := &SettingsWindow{Model: m}
	tt := uiNewTester(s.View, 720, 720)
	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	b := switchRect(t, tt, "自动检查更新")
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	if m.autoChecks != 1 || !m.autoSet {
		t.Errorf("auto check set %d times, on = %v", m.autoChecks, m.autoSet)
	}
}

// fakeStatsModel serves the stats window.
type fakeStatsModel struct {
	today, goal int
	week, month []store.DayCount
	streak      int
	highest     int
	avg         float64
}

func (f *fakeStatsModel) TodayCount() int                 { return f.today }
func (f *fakeStatsModel) DailyGoal() int                  { return f.goal }
func (f *fakeStatsModel) WeekStats() []store.DayCount     { return f.week }
func (f *fakeStatsModel) MonthStats() []store.DayCount    { return f.month }
func (f *fakeStatsModel) ConsecutiveDays() int            { return f.streak }
func (f *fakeStatsModel) HighestRecord() int              { return f.highest }
func (f *fakeStatsModel) AverageDaily() float64           { return f.avg }
func (f *fakeStatsModel) Badges() (bool, bool, int, bool) { return true, false, 12, true }

func TestStatsView(t *testing.T) {
	now := time.Now()
	week := make([]store.DayCount, 7)
	for i := range week {
		day := now.AddDate(0, 0, i-6)
		week[i] = store.DayCount{Day: day, Count: (i % 4) * 2}
	}
	m := &fakeStatsModel{
		today: 5, goal: 8, week: week, month: week,
		streak: 4, highest: 9, avg: 4.4,
	}
	tt := uiNewTester((&StatsWindow{Model: m}).View, 680, 640)
	for _, want := range []string{"今日", "连续天数", "最高纪录", "最近 7 天", "成就", "已达成", "未达成"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing; texts %q", want, tt.Texts())
		}
	}
}

// fakeAlertModel serves the reminder popup.
type fakeAlertModel struct {
	msg     string
	today   int
	goal    int
	denied  int
	drank   int
	snoozed int
}

func (f *fakeAlertModel) HydrationMessage() string { return f.msg }
func (f *fakeAlertModel) TodayCount() int          { return f.today }
func (f *fakeAlertModel) DailyGoal() int           { return f.goal }
func (f *fakeAlertModel) RecordDrinkAndClose()     { f.drank++ }
func (f *fakeAlertModel) DrinkDenial() int         { return f.denied }
func (f *fakeAlertModel) SnoozeAndClose()          { f.snoozed++ }

func TestHydrationAlertView(t *testing.T) {
	m := &fakeAlertModel{msg: "喝水时间到！补充水分，保持活力。", today: 2, goal: 8}
	tt := uiNewTester((&HydrationAlert{Model: m}).View, 380, 460)
	for _, want := range []string{"该喝水了！", "喝水时间到", "已喝水", "10 分钟后再提醒"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("请等待") {
		t.Errorf("denial hint without a denial; texts %q", tt.Texts())
	}
	if err := tt.Click("已喝水"); err != nil {
		t.Fatal(err)
	}
	if m.drank != 1 {
		t.Errorf("drank = %d", m.drank)
	}
	if err := tt.Click("10 分钟后再提醒"); err != nil {
		t.Fatal(err)
	}
	if m.snoozed != 1 {
		t.Errorf("snoozed = %d", m.snoozed)
	}
}

func TestHydrationAlertViewDenial(t *testing.T) {
	m := &fakeAlertModel{msg: "喝水时间到！", today: 2, goal: 8, denied: 20}
	tt := uiNewTester((&HydrationAlert{Model: m}).View, 380, 460)
	if !tt.HasText("请等待 20 秒后再试") {
		t.Errorf("denial hint missing; texts %q", tt.Texts())
	}
}

// fakeLockModel serves the rest overlay, recording the way out.
type fakeLockModel struct {
	remaining time.Duration
	ended     int
}

func (f *fakeLockModel) RestRemaining() time.Duration { return f.remaining }
func (f *fakeLockModel) RestTotal() time.Duration     { return 5 * time.Minute }
func (f *fakeLockModel) EndRest()                     { f.ended++ }

func TestLockOverlayView(t *testing.T) {
	m := &fakeLockModel{remaining: 4*time.Minute + 30*time.Second}
	tt := uiNewTester((&LockOverlay{Model: m}).View, 800, 600)
	for _, want := range []string{"休息时间", "04:30", "强制退出休息"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("强制退出休息"); err != nil {
		t.Fatal(err)
	}
	if m.ended != 1 {
		t.Errorf("ended = %d", m.ended)
	}
}

// Once the rest has run its course the overlay offers the way on, and the
// countdown reads zero.
func TestLockOverlayDone(t *testing.T) {
	m := &fakeLockModel{}
	tt := uiNewTester((&LockOverlay{Model: m}).View, 800, 600)
	for _, want := range []string{"完成休息，继续工作", "休息完成，随时可以继续"} {
		if !tt.HasText(want) {
			t.Errorf("done: %q missing; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("完成休息，继续工作"); err != nil {
		t.Fatal(err)
	}
	if m.ended != 1 {
		t.Errorf("ended = %d", m.ended)
	}
}

// The rest overlay stays dark whatever the system appearance: it covers a
// screen.
func TestLockOverlayStaysDark(t *testing.T) {
	for _, dark := range []bool{false, true} {
		m := &fakeLockModel{remaining: time.Minute}
		tt := uiNewTester((&LockOverlay{Model: m}).View, 800, 600)
		tt.SetDark(dark)
		tt.Advance(600 * time.Millisecond)
		tt.Frame()
		img := tt.Image()
		r, g, b, _ := img.At(20, 20).RGBA()
		if int(r>>8)+int(g>>8)+int(b>>8) > 120 {
			t.Errorf("dark=%v: the overlay's corner is %d,%d,%d — it is not dark", dark, r>>8, g>>8, b>>8)
		}
	}
}
