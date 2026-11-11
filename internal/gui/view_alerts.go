package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"restify/internal/core"
)

// HydrationAlert is the drink reminder popup: the message, the day's
// progress and the two actions.
type HydrationAlert struct {
	Model HydrationAlertModel
}

// View builds the popup's content: the water mark, the reminder, a card
// with the message and the day's count, the health notice, and the two
// choices at the bottom.
func (a *HydrationAlert) View(c *ui.Context) {
	tk := applyTheme(c)
	m := a.Model

	ui.Column(c).Fill().Padding(24).Gap(16).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		// The popup's own watermark: the droplet on the water accent's wash,
		// larger than the panel's brand mark, on its corner radius.
		ui.Row(c).Size(64, 64).Radius(64 * 0.32).Background(AccentWater.Fill(tk)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			display.Icon(c, iconDrop, display.IconProps{Size: display.IconLarge}).
				FontSize(34).TextColor(AccentWater.Color(tk))
		})

		display.Text(c, "该喝水了！", display.TextProps{Strong: true, FontSize: 22}).SingleLine()

		// The message and the day's progress share one soft block, so the
		// reminder reads as one statement.
		layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).FillWidth().Padding(12, 14).Gap(8).
			Children(func() {
				display.Text(c, m.HydrationMessage(), display.TextProps{FontSize: 13}).TextAlign(ui.Center)
				display.Text(c, fmt.Sprintf("今日 %d/%d 杯", m.TodayCount(), m.DailyGoal()),
					display.TextProps{Type: display.TextSecondary, FontSize: 12}).
					FontFeatures("tnum").TextAlign(ui.Center)
			})

		Notice(c, "成年人每天应该喝 1.5-2 升水，保持水分充足。")

		// The two choices close the popup, one under the other — unless
		// the record came too soon after the last, which keeps it up.
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			drank := action.Button(c, "已喝水", action.ButtonProps{Variant: action.Primary, Size: theme.Middle})
			drank.Label("记录一次喝水并关闭")
			if drank.Clicked() {
				m.RecordDrinkAndClose()
			}
			if action.Button(c, "10 分钟后再提醒", action.ButtonProps{Variant: action.Default, Size: theme.Middle}).Clicked() {
				m.SnoozeAndClose()
			}
			if wait := m.DrinkDenial(); wait > 0 {
				feedback.Alert(c, fmt.Sprintf("记录太快啦，请等待 %d 秒后再试", wait),
					feedback.AlertProps{Type: feedback.Warning, Variant: feedback.Soft})
				c.After(time.Second) // the wait counts down
			}
		})
	})
}

// LockOverlay is the forced-rest cover of one screen: the countdown and
// the way out. It keeps the dark look whatever the theme is — it covers
// the screen the way a rest screen does.
type LockOverlay struct {
	Model LockModel
}

// View builds one screen's cover. The app's rest session redraws it every
// second as the countdown moves.
func (l *LockOverlay) View(c *ui.Context) {
	tk := applyDarkTheme(c)
	m := l.Model
	remaining := m.RestRemaining()

	ui.Column(c).Fill().Padding(48).Gap(20).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		// The moon on a soft wash of the rest accent, as Lobe's result
		// marks an outcome.
		ui.Row(c).Size(72, 72).Radius(72 * 0.32).Background(AccentRest.Fill(tk)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			display.Icon(c, iconMoon, display.IconProps{Size: display.IconLarge}).
				FontSize(34).TextColor(AccentRest.Color(tk))
		})

		display.Text(c, "休息时间", display.TextProps{Strong: true, FontSize: 28}).SingleLine()

		display.Text(c, "请休息片刻，眺望远方、活动身体、深呼吸",
			display.TextProps{Type: display.TextSecondary, FontSize: 14}).
			TextAlign(ui.Center).MaxWidth(420)

		// The countdown breathes: it brightens and eases over four
		// seconds, quietly, the way resting time passes.
		clock := display.Text(c, core.FormatClock(remaining),
			display.TextProps{Strong: true, FontSize: 64}).FontFeatures("tnum").SingleLine()
		cycle := clock.Loop("breath", 4*time.Second, ui.Linear)
		breath := cycle
		if breath > 0.5 {
			breath = 1 - breath
		}
		clock.Opacity(0.62 + 0.76*breath)

		if remaining > 0 {
			display.Text(c, "倒计时结束后自动恢复工作",
				display.TextProps{Type: display.TextSecondary, FontSize: 12}).SingleLine()
		} else {
			display.Text(c, "休息完成，随时可以继续",
				display.TextProps{Type: display.TextSecondary, FontSize: 12}).SingleLine()
		}

		if remaining > 0 {
			if action.Button(c, "强制退出休息", action.ButtonProps{Variant: action.Default, Size: theme.Middle}).Clicked() {
				m.EndRest()
			}
		} else {
			// The way out arrives softly once the rest has run its
			// course.
			done := action.Button(c, "完成休息，继续工作",
				action.ButtonProps{Variant: action.Primary, Size: theme.Middle})
			done.Label("结束休息").Transition(ui.ElementTransition{
				Animation: &ui.Animation{Spring: ui.Smooth.Spring},
				Enter:     &ui.Motion{},
			})
			if done.Clicked() {
				m.EndRest()
			}
		}
	})
}
