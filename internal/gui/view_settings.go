package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"restify/internal/core"
	"restify/internal/store"
)

// SettingsWindow is the settings: a sidebar of pages over a scrolling
// body, in the management-window skeleton of the design language.
type SettingsWindow struct {
	Model SettingsModel

	page string // the selected page id
	// scroll is the body's scroll state; scrollPage names the page it
	// belongs to, so another page opens at its top.
	scroll     ui.ScrollState
	scrollPage string
	// Drafts of the message-library add fields, kept across frames.
	breakDraft     string
	hydrationDraft string
}

// settingsSidebarWidth is the settings sidebar's width. The design's
// management skeleton says 240, for sidebars with longer labels and
// trailing values; Restify's four two-character pages need much less, and
// a wide sidebar of short items reads as empty.
const settingsSidebarWidth = 176

// pages in sidebar order; ids are stable keys.
var settingsPages = []struct {
	id, title string
	icon      *ui.SVG
}{
	{"work", "工作", iconTimer},
	{"hydration", "喝水", iconDrop},
	{"general", "通用", iconSettings},
	{"about", "关于", iconInfo},
}

// View builds the window's content.
func (s *SettingsWindow) View(c *ui.Context) {
	tk := applyTheme(c)
	if s.page == "" {
		s.page = settingsPages[0].id
	}
	bar := c.TitleBar()

	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(tk.BgLayout).Children(func() {
		ui.Column(c).Width(settingsSidebarWidth).
			BorderWidth(0, 1, 0, 0).BorderColor(tk.BorderSecondary).Children(func() {
			// Room for the traffic lights above the pages.
			ui.Box(c).Height(max(bar.Height, 28))
			ui.Column(c).Padding(8).Children(func() {
				if clicked := display.List(c, display.ListProps{
					Items:  sidebarItems(),
					Active: s.page,
				}); clicked != "" {
					s.page = clicked
				}
			})
		})
		ui.Column(c).Grow(1).Basis(0).Background(tk.BgContainer).Children(func() {
			s.pageBody(c, tk)
		})
	})
}

func sidebarItems() []display.ListItem {
	items := make([]display.ListItem, len(settingsPages))
	for i, pg := range settingsPages {
		items[i] = display.ListItem{Key: pg.id, Label: pg.title, Icon: pg.icon}
	}
	return items
}

func (s *SettingsWindow) pageBody(c *ui.Context, tk *theme.Tokens) {
	if s.scrollPage != s.page {
		s.scroll, s.scrollPage = ui.ScrollState{}, s.page
	}
	body := layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true, State: &s.scroll}, func() {
		ui.Column(c).Padding(0, 24, 24, 24).Gap(16).Children(func() {
			DragStrip(c)
			switch s.page {
			case "work":
				s.workPage(c)
			case "hydration":
				s.hydrationPage(c, tk)
			case "general":
				s.generalPage(c)
			case "about":
				s.aboutPage(c)
			}
		})
	})
	body.Fill()
}

// sliderRow lays a slider at the right of its label, beside the value it
// set. It reports the change in the same frame, while value still points
// at the caller's variable.
func sliderRow(c *ui.Context, label, detail string, value *float64, lo, hi float64, unit string, apply func()) {
	before := *value
	ui.Row(c).Gap(16).Padding(10, 0).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			display.Text(c, label, display.TextProps{})
			if detail != "" {
				display.Text(c, detail, display.TextProps{Type: display.TextSecondary, FontSize: 12}).MaxLines(2)
			}
		})
		choice.Slider(c, value, choice.SliderProps{Min: lo, Max: hi, Step: 1}).Label(label).Width(140)
		display.Text(c, sliderAmount(*value, unit), display.TextProps{FontSize: 13}).
			FontFeatures("tnum").Width(52).SingleLine()
	})
	if *value != before {
		apply()
	}
}

func sliderAmount(value float64, unit string) string {
	if unit == "%" {
		return fmt.Sprintf("%.0f%%", value)
	}
	return fmt.Sprintf("%.0f %s", value, unit)
}

// switchRow lays a switch at the right of its label and applies the state
// it moved to.
func switchRow(c *ui.Context, label, detail string, on bool, apply func(on bool)) {
	changed := on
	SettingRow(c, label, detail, func() {
		choice.Switch(c, &changed, choice.SwitchProps{Size: theme.Middle}).Label(label)
	})
	if changed != on {
		apply(changed)
	}
}

// picksRow lays the presets of a setting as buttons beside its label, the
// one the setting holds filled; active is -1 when it holds none of them.
func picksRow(c *ui.Context, label string, options []string, active int, choose func(i int)) {
	SettingRow(c, label, "", func() {
		for i, option := range options {
			variant := action.Default
			if i == active {
				variant = action.Primary
			}
			if action.Button(c, option, action.ButtonProps{Variant: variant, Size: theme.Small}).Clicked() {
				choose(i)
			}
		}
	})
}

// durationPicksRow is picksRow for quick picks of a duration, labelled in
// whole minutes.
func durationPicksRow(c *ui.Context, label string, choices []time.Duration, current time.Duration, choose func(d time.Duration)) {
	options := make([]string, len(choices))
	for i, d := range choices {
		options[i] = fmt.Sprintf("%d 分钟", int(d/time.Minute))
	}
	picksRow(c, label, options, indexOfDuration(current, choices), func(i int) {
		choose(choices[i])
	})
}

func indexOfDuration(d time.Duration, choices []time.Duration) int {
	for i, choice := range choices {
		if d == choice {
			return i
		}
	}
	return -1
}

func indexOfInt(n int, choices []int) int {
	for i, choice := range choices {
		if n == choice {
			return i
		}
	}
	return -1
}

// ---- Work page ----

var (
	workDurations = []time.Duration{15 * time.Minute, 25 * time.Minute, 45 * time.Minute, time.Hour}
	restDurations = []time.Duration{3 * time.Minute, 5 * time.Minute, 10 * time.Minute}
	goalChoices   = []int{6, 8, 10, 12}
)

func (s *SettingsWindow) workPage(c *ui.Context) {
	m := s.Model
	set := m.Settings()

	SettingsCard(c, "工作时长", func() {
		minutes := set.WorkDuration.Minutes()
		sliderRow(c, "工作时长", "计时结束后提醒休息", &minutes, 5, 60, "分钟", func() {
			m.SetSetting(func(x *store.Settings) { x.WorkDuration = time.Duration(minutes*60) * time.Second })
		})
		RowSeparator(c)
		durationPicksRow(c, "快速选择", workDurations, set.WorkDuration, func(d time.Duration) {
			m.SetSetting(func(x *store.Settings) { x.WorkDuration = d })
		})
		RowSeparator(c)
		SettingRow(c, "预计休息时间", "现在开始计时，到点提醒休息", func() {
			at := time.Now().Add(set.WorkDuration)
			display.Text(c, at.Format("15:04"), display.TextProps{FontSize: 13}).FontFeatures("tnum").SingleLine()
		})
	})

	SettingsCard(c, "休息设置", func() {
		grace := set.GracePeriod.Minutes()
		sliderRow(c, "休息时长", "强制休息的持续时间", &grace, 1, 30, "分钟", func() {
			m.SetSetting(func(x *store.Settings) { x.GracePeriod = time.Duration(grace*60) * time.Second })
		})
		RowSeparator(c)
		durationPicksRow(c, "快速选择", restDurations, set.GracePeriod, func(d time.Duration) {
			m.SetSetting(func(x *store.Settings) { x.GracePeriod = d })
		})
		RowSeparator(c)
		switchRow(c, "强制休息", "计时结束后全屏提示休息，倒计时结束自动恢复", set.EnableAutoLock, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnableAutoLock = on })
		})
		RowSeparator(c)
		switchRow(c, "启动时自动开始计时", "Restify 启动后立即开始工作计时", set.AutoStartTimer, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.AutoStartTimer = on })
		})
	})

	SettingsCard(c, "智能工作提醒", func() {
		switchRow(c, "检测持续工作", "未计时时检测到持续工作则提醒开启计时器", set.EnableSmartReminder, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnableSmartReminder = on })
		})
		RowSeparator(c)
		threshold := set.SmartReminderThreshold.Minutes()
		sliderRow(c, "检测阈值", "持续工作多久后提醒", &threshold, 3, 30, "分钟", func() {
			m.SetSetting(func(x *store.Settings) { x.SmartReminderThreshold = time.Duration(threshold*60) * time.Second })
		})
	})

	Notice(c, "每工作 "+core.FormatShort(set.WorkDuration)+"，休息 "+core.FormatShort(set.GracePeriod)+"，保持节奏。")
}

// ---- Hydration page ----

var intervalChoices = []struct {
	label string
	d     time.Duration
}{
	{"1 分钟", time.Minute},
	{"30 分钟", 30 * time.Minute},
	{"1 小时", time.Hour},
	{"1.5 小时", 90 * time.Minute},
	{"2 小时", 2 * time.Hour},
}

func intervalOptions() []field.SelectOption {
	out := make([]field.SelectOption, len(intervalChoices))
	for i, choice := range intervalChoices {
		out[i] = field.SelectOption{Value: choice.label, Label: choice.label}
	}
	return out
}

// intervalLabel is the interval's choice, and 1 小时 for one the presets do
// not name — the page shows the nearest preset, as the pills did.
func intervalLabel(d time.Duration) string {
	for _, choice := range intervalChoices {
		if choice.d == d {
			return choice.label
		}
	}
	return intervalChoices[2].label
}

func intervalDuration(label string) time.Duration {
	for _, choice := range intervalChoices {
		if choice.label == label {
			return choice.d
		}
	}
	return intervalChoices[2].d
}

func (s *SettingsWindow) hydrationPage(c *ui.Context, tk *theme.Tokens) {
	m := s.Model
	set := m.Settings()

	SettingsCard(c, "每日目标", func() {
		goal := float64(set.DailyGoal)
		SettingRow(c, "目标杯数", "成年人每天建议饮水 1.5-2 升", func() {
			field.InputNumber(c, &goal, field.InputNumberProps{Min: 1, Max: 20, Step: 1}).
				Label("目标杯数").Width(56)
		})
		if int(goal) != set.DailyGoal {
			m.SetSetting(func(x *store.Settings) { x.DailyGoal = int(goal) })
		}
		RowSeparator(c)
		options := make([]string, len(goalChoices))
		for i, n := range goalChoices {
			options[i] = fmt.Sprint(n)
		}
		picksRow(c, "快速选择", options, indexOfInt(set.DailyGoal, goalChoices), func(i int) {
			m.SetSetting(func(x *store.Settings) { x.DailyGoal = goalChoices[i] })
		})
	})

	SettingsCard(c, "提醒", func() {
		switchRow(c, "喝水提醒", "按固定间隔提醒喝水", set.EnableReminder, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnableReminder = on })
		})
		RowSeparator(c)
		SettingRow(c, "提醒间隔", "", func() {
			selected := intervalLabel(set.ReminderInterval)
			field.Select(c, &selected, field.SelectProps{Options: intervalOptions(), Size: theme.Middle}).
				Label("提醒间隔")
			if d := intervalDuration(selected); d != set.ReminderInterval {
				m.SetSetting(func(x *store.Settings) { x.ReminderInterval = d })
			}
		})
	})

	Panel(c, "今日饮水", func() {
		if opener, ok := m.(interface{ OpenStats() }); ok {
			if action.Button(c, "查看统计", action.ButtonProps{Variant: action.Link, Size: theme.Small}).Clicked() {
				opener.OpenStats()
			}
		}
	}, func() {
		BigValue(c, "已记录", fmt.Sprint(m.TodayCount()), fmt.Sprintf("/ %d 杯", set.DailyGoal), AccentWater.Color(tk))
		if set.DailyGoal > 0 {
			ProgressBar(c, float64(m.TodayCount())/float64(set.DailyGoal), AccentWater.Color(tk))
		}
	})
}

// ---- General page ----

func (s *SettingsWindow) generalPage(c *ui.Context) {
	m := s.Model
	set := m.Settings()

	SettingsCard(c, "启动", func() {
		switchRow(c, "登录时启动", "开机自动运行 Restify", set.LaunchAtLogin, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.LaunchAtLogin = on })
			m.ApplyLaunchAtLogin(on)
		})
		RowSeparator(c)
		switchRow(c, "显示菜单栏图标", "关闭后菜单栏不再显示倒计时", set.ShowMenuBarIcon, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.ShowMenuBarIcon = on })
		})
	})

	SettingsCard(c, "提醒方式", func() {
		switchRow(c, "提醒声音", "提醒时播放声音", set.EnableSound, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnableSound = on })
		})
		RowSeparator(c)
		volume := set.SoundVolume * 100
		sliderRow(c, "音量", "", &volume, 0, 100, "%", func() {
			m.SetSetting(func(x *store.Settings) { x.SoundVolume = volume / 100 })
		})
		RowSeparator(c)
		switchRow(c, "应用内提醒窗口", "提醒时在屏幕上弹出提醒窗口", set.EnablePopupAlert, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnablePopupAlert = on })
		})
		RowSeparator(c)
		// The row's hint gives way to the last test's outcome for a
		// while, so a click always says what happened.
		detail := "发送一条测试通知检查权限"
		if text, at := m.NotificationNotice(); text != "" && c.Now().Sub(at) < NoticeFlash {
			detail = text
			if left := NoticeFlash - c.Now().Sub(at); left > 0 {
				c.After(left)
			}
		}
		SettingRow(c, "通知", detail, func() {
			if action.Button(c, "发送测试通知", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				m.SendTestNotification()
			}
		})
	})

	SettingsCard(c, "自定义声音", func() {
		s.soundRow(c, "break", "休息提醒音", set.CustomBreakSoundPath)
		RowSeparator(c)
		s.soundRow(c, "hydration", "喝水提醒音", set.CustomHydrationSoundPath)
	})

	SettingsCard(c, "提醒文案", func() {
		switchRow(c, "激励性文案", "休息提醒随机使用激励性语句", set.EnableMotivationalMessages, func(on bool) {
			m.SetSetting(func(x *store.Settings) { x.EnableMotivationalMessages = on })
		})
		RowSeparator(c)
		s.messageLibrary(c, m, "break", "休息文案库", set.BreakMessageLibrary, &s.breakDraft)
		RowSeparator(c)
		s.messageLibrary(c, m, "hydration", "喝水文案库", set.HydrationMessageLibrary, &s.hydrationDraft)
	})

	SettingsCard(c, "外观", func() {
		themeSel := themeIndex(set.ThemeMode)
		SettingRow(c, "主题", "", func() {
			action.Segmented(c, &themeSel, []string{"跟随系统", "浅色", "深色"},
				action.SegmentedProps{Size: theme.Small}).Label("主题")
		})
		if themeSel != themeIndex(set.ThemeMode) {
			tm := []store.ThemeMode{store.ThemeSystem, store.ThemeLight, store.ThemeDark}[themeSel]
			m.SetSetting(func(x *store.Settings) { x.ThemeMode = tm })
		}
		RowSeparator(c)
		iconSel := iconIndex(set.IconStyle)
		ui.Column(c).Gap(8).Padding(10, 0).Children(func() {
			display.Text(c, "菜单栏图标", display.TextProps{})
			action.Segmented(c, &iconSel, []string{"计时器", "月亮", "时钟", "沙漏", "水滴"},
				action.SegmentedProps{Size: theme.Small, Block: true}).Label("菜单栏图标")
		})
		if iconSel != iconIndex(set.IconStyle) {
			is := []store.IconStyle{store.IconTimer, store.IconMoon, store.IconClock, store.IconHourglass, store.IconDrop}[iconSel]
			m.SetSetting(func(x *store.Settings) { x.IconStyle = is })
		}
	})

	SettingsCard(c, "数据", func() {
		SettingRow(c, "导出数据", "设置与喝水记录导出为 JSON 文件", func() {
			if action.Button(c, "导出", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				m.ExportData()
			}
		})
		RowSeparator(c)
		SettingRow(c, "清除数据", "删除全部设置与喝水记录", func() {
			if action.Button(c, "清除全部", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
				m.ClearAllData()
			}
		})
	})
}

// soundRow shows one reminder sound's path, with the buttons that import
// another, audition it and clear it. The audition button says what a
// click does: 试听 when silent, 暂停 while it plays, 继续 while it waits
// paused.
func (s *SettingsWindow) soundRow(c *ui.Context, kind, label, path string) {
	m := s.Model
	name := "默认（系统提示音）"
	if path != "" {
		name = path
	}
	audition := "试听"
	if playing, paused := m.SoundPreviewState(kind); paused {
		audition = "继续"
	} else if playing {
		audition = "暂停"
	}
	SettingRow(c, label, name, func() {
		if action.Button(c, "导入", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() {
			if p := m.ImportSound(kind); p != "" {
				m.SetSetting(func(x *store.Settings) {
					if kind == "break" {
						x.CustomBreakSoundPath = p
					} else {
						x.CustomHydrationSoundPath = p
					}
				})
			}
		}
		if action.Button(c, audition, action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
			m.TestSound(kind)
		}
		clearButton := action.ActionIcon(c, iconClose, action.ActionIconProps{
			Variant: action.ActionIconBorderless,
			Size:    theme.Small,
			Title:   "清除",
		})
		clearButton.Disabled(path == "")
		if clearButton.Clicked() {
			m.SetSetting(func(x *store.Settings) {
				if kind == "break" {
					x.CustomBreakSoundPath = ""
				} else {
					x.CustomHydrationSoundPath = ""
				}
			})
		}
	})
}

// messageLibrary lists the library's messages with remove buttons, an add
// field and a reset button, in an inset card. The draft lives on the
// window so typing survives frames.
func (s *SettingsWindow) messageLibrary(c *ui.Context, m SettingsModel, kind, title string, library []string, draft *string) {
	ui.Column(c).Gap(6).Padding(10, 0).Children(func() {
		display.Text(c, title, display.TextProps{FontSize: 13, Strong: true}).MaxLines(1)
		layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined}).Padding(6).Gap(4).Children(func() {
			if len(library) == 0 {
				display.Text(c, "文案库为空，将使用默认文案", display.TextProps{Type: display.TextSecondary, FontSize: 11}).Padding(4)
			}
			for i, msg := range library {
				index := i
				ui.Row(c).Key(fmt.Sprintf("%s-msg-%d", kind, index)).Gap(8).AlignItems(ui.Center).
					Padding(4, 8).Children(func() {
					display.Text(c, msg, display.TextProps{FontSize: 12}).Grow(1).Basis(0).MaxLines(2)
					if action.ActionIcon(c, iconClose, action.ActionIconProps{
						Variant: action.ActionIconBorderless,
						Size:    theme.Small,
						Title:   "删除这条文案",
					}).Clicked() {
						m.RemoveMessage(kind, index)
					}
				})
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					field.Input(c, draft, field.InputProps{Size: theme.Small, Placeholder: "添加一条文案…"})
				})
				if action.Button(c, "添加", action.ButtonProps{Variant: action.Default, Size: theme.Small}).Clicked() && *draft != "" {
					m.AddMessage(kind, *draft)
					*draft = ""
				}
				if action.Button(c, "重置", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					m.ResetMessages(kind)
				}
			})
		})
	})
}

func themeIndex(m store.ThemeMode) int {
	switch m {
	case store.ThemeLight:
		return 1
	case store.ThemeDark:
		return 2
	default:
		return 0
	}
}

func iconIndex(s store.IconStyle) int {
	switch s {
	case store.IconMoon:
		return 1
	case store.IconClock:
		return 2
	case store.IconHourglass:
		return 3
	case store.IconDrop:
		return 4
	default:
		return 0
	}
}

// ---- About page ----

func (s *SettingsWindow) aboutPage(c *ui.Context) {
	m := s.Model
	Card(c, func() {
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			Brand(c, 44)
			ui.Column(c).Gap(2).Children(func() {
				display.Text(c, "Restify", display.TextProps{Strong: true, FontSize: 17})
				display.Text(c, "休息与喝水提醒", display.TextProps{Type: display.TextSecondary, FontSize: 12})
			})
		})
		Hairline(c)
		KV(c, "版本", m.Version())
		KV(c, "构建", "Go + mygo")
	})
	set := m.Settings()
	Notice(c, "工作 "+core.FormatShort(set.WorkDuration)+" · 休息 "+core.FormatShort(set.GracePeriod)+" · 目标 "+fmt.Sprint(set.DailyGoal)+" 杯水。")
}
