package gui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"restify/internal/store"
)

// StatsWindow shows today, the week and month charts and the achievement
// badges.
type StatsWindow struct {
	Model StatsModel
}

// chartHeight is the room a bar chart gives its tallest bar, in DIPs.
const chartHeight = 110

// View builds the window's content: the day, the two charts and the badges
// on Lobe blocks, in one scrolling page under the drag strip.
func (s *StatsWindow) View(c *ui.Context) {
	tk := applyTheme(c)
	m := s.Model

	layout.ScrollArea(c, layout.ScrollAreaProps{Fade: true}, func() {
		ui.Column(c).Padding(0, 24, 24, 24).Gap(16).Children(func() {
			DragStrip(c)

			today, goal := m.TodayCount(), m.DailyGoal()
			Card(c, func() {
				statHeading(c, tk, iconDrop, AccentWater, "今日", 120*time.Millisecond)
				BigValue(c, "已记录", strconv.Itoa(today), fmt.Sprintf("/ %d 杯", goal), AccentWater.Color(tk))
				if goal > 0 {
					ProgressBar(c, float64(today)/float64(goal), AccentWater.Color(tk))
				}
				Metrics(c,
					Metric{Label: "连续天数", Value: strconv.Itoa(m.ConsecutiveDays())},
					Metric{Label: "最高纪录", Value: strconv.Itoa(m.HighestRecord())},
					Metric{Label: "7 天平均", Value: fmt.Sprintf("%.1f 杯", m.AverageDaily())},
				)
			}).Key("stats-today").Transition(enter(0))

			Card(c, func() {
				statHeading(c, tk, iconChart, AccentChart, "最近 7 天", 160*time.Millisecond)
				weekBars(c, tk, m.WeekStats(), goal, AccentChart.Color(tk))
			}).Key("stats-week").Transition(enter(40 * time.Millisecond))

			Card(c, func() {
				statHeading(c, tk, iconCalendar, AccentChart, "本月 · "+time.Now().Format("2006年1月"), 200*time.Millisecond)
				weekBars(c, tk, m.MonthStats(), goal, AccentChart.Color(tk))
			}).Key("stats-month").Transition(enter(80 * time.Millisecond))

			Card(c, func() {
				statHeading(c, tk, iconMedal, AccentWarn, "成就", 240*time.Millisecond)
				s.badges(c, tk, m)
			}).Key("stats-badges").Transition(enter(120 * time.Millisecond))
		})
	}).Fill()
}

// statHeading heads a card with the icon that names its section, colored
// by the section's accent, so each block says what it is at a glance. The
// icon pops in a beat after its card lands, on the bouncy spring.
func statHeading(c *ui.Context, tk *theme.Tokens, icon *ui.SVG, a Accent, title string, iconDelay time.Duration) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ic := display.Icon(c, icon, display.IconProps{Size: display.IconMiddle}).TextColor(a.Color(tk))
		popIn(ic, iconDelay)
		display.Text(c, title, display.TextProps{Strong: true, FontSize: 15})
	})
}

// weekBars draws one bar per day in a soft well of the section's color,
// oldest to newest, the count over each bar and the weekday initial under
// it; the bars scale to the goal or the best day, whichever is higher.
//
// There is no chart in Lobe UI, so the bars are boxes laid out by hand and
// colored from the tokens; the weekday row sits outside the well, on the
// same columns, to keep the initials under their bars.
func weekBars(c *ui.Context, tk *theme.Tokens, days []store.DayCount, goal int, col ui.Color) {
	maxv := max(goal, 1)
	for _, d := range days {
		if d.Count > maxv {
			maxv = d.Count
		}
	}

	// The label row repeats the well's horizontal padding and column gap,
	// so its initials sit under their own bars.
	const barGap, wellPadX = 4, float32(12)
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).FillWidth().Padding(10, wellPadX).Radius(tk.BorderRadius).Background(col.Alpha(0.10)).
			Gap(barGap).AlignItems(ui.End).Children(func() {
			for _, d := range days {
				ui.Column(c).Grow(1).Basis(0).Gap(3).AlignItems(ui.Center).Children(func() {
					if d.Count > 0 {
						display.Text(c, strconv.Itoa(d.Count), display.TextProps{FontSize: 10}).
							TextColor(tk.TextDescription).FontFeatures("tnum").SingleLine()
					}
					h := chartHeight * float32(d.Count) / float32(maxv)
					if d.Count > 0 && h < 3 {
						h = 3
					}
					ui.Box(c).Height(h).MinWidth(6).Radius(2).Background(col)
				})
			}
		})

		ui.Row(c).Padding(0, wellPadX).Gap(barGap).Children(func() {
			for _, d := range days {
				display.Text(c, weekdayInitial(d.Day), display.TextProps{FontSize: 9}).
					TextColor(tk.TextDescription).TextAlign(ui.Center).SingleLine().
					Grow(1).Basis(0)
			}
		})
	})
}

// weekdayInitials are the one-character names of the days, Monday first.
var weekdayInitials = [7]string{"一", "二", "三", "四", "五", "六", "日"}

// weekdayInitial names the day under its bar.
func weekdayInitial(day time.Time) string {
	return weekdayInitials[(int(day.Weekday())+6)%7]
}

// badges lists the achievements, each a filled block: its icon, name,
// detail and whether it is earned, colored only by its own accent. The
// icons pop in one after another after their card lands.
func (s *StatsWindow) badges(c *ui.Context, tk *theme.Tokens, m StatsModel) {
	streak7, goal30, goalDays, highest := m.Badges()
	ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
		badge(c, tk, iconFlame, AccentWarn, "连续 7 天", "每天至少喝一次水", streak7, 240*time.Millisecond)
		badge(c, tk, iconTrophy, AccentChart, "30 天达标", fmt.Sprintf("累计 %d 天完成目标", goalDays), goal30, 300*time.Millisecond)
		badge(c, tk, iconMedal, AccentRest, "单日纪录", fmt.Sprintf("一天喝满 %d 杯", m.HighestRecord()), highest, 360*time.Millisecond)
	})
}

// badge is one achievement: a soft tile whose icon and name take the
// achievement's color once it is earned, and its tag says so either way.
func badge(c *ui.Context, tk *theme.Tokens, icon *ui.SVG, a Accent, title, detail string, earned bool, iconDelay time.Duration) {
	fg, titleCol := tk.TextTertiary, tk.TextDescription
	if earned {
		fg = a.Color(tk)
		titleCol = fg
	}
	layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(12).Gap(8).
		Grow(1).Basis(0).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Size(32, 32).Radius(tk.BorderRadius).Background(a.Fill(tk)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ic := display.Icon(c, icon, display.IconProps{Size: display.IconMiddle}).TextColor(fg)
			popIn(ic, iconDelay)
		})
		display.Text(c, title, display.TextProps{Strong: true, FontSize: 13}).
			TextColor(titleCol).SingleLine()
		display.Text(c, detail, display.TextProps{Type: display.TextSecondary, FontSize: 11, Rows: 2}).
			TextAlign(ui.Center)
		if earned {
			Tag(c, "已达成", a.Tag())
		} else {
			Tag(c, "未达成", "")
		}
	})
}
