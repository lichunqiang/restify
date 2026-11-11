package gui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"restify/internal/core"
)

// The panel's content height by state, in DIPs at its 340 width: the
// running countdown opens the work card taller than its idle summary,
// and a feedback line shows under the record button for a moment.
// TestPanelHeights holds these to what the layout really measures.
const (
	panelIdleHeight = 416
	panelRunHeight  = 487
	panelFeedback   = 18
)

// PanelHeight is the height the panel's content asks for in a state: the
// height of its phase's card, and the feedback line's while one shows.
// It sizes the flyout at a glance; the panel then tracks its settled
// content and corrects it through Popover.OnHeight.
func PanelHeight(phase core.Phase, feedback bool) int {
	h := panelIdleHeight
	if phase != core.PhaseIdle {
		h = panelRunHeight
	}
	if feedback {
		h += panelFeedback
	}
	return h
}

// panelPad is the root column's bottom padding, the margin the window
// height carries under the footer.
const panelPad = 12

// Popover is the menu-bar panel: the work timer, today's drinks and the
// quick actions, in the material of the popover it shows in.
//
// The work card is state-aware: idle it is compact — a summary line and
// the one button that matters — and only as the timer runs does it open
// into the big countdown, progress bar and controls. The sections enter
// one under another when the panel opens, the countdown pulses as the
// phase moves, and the hydration ring bounces as a drink is recorded.
type Popover struct {
	Model PopoverModel

	// OnHeight, when set, hears the height the content settles at, in
	// DIPs, as the panel's state moves it — the app points it at the
	// flyout's resize. Nil in tests that measure the view alone.
	OnHeight func(height int)

	// lastPhase and phaseAt drive the countdown's pulse.
	lastPhase core.Phase
	phaseAt   time.Time

	// Height tracking: trackSig names the state that laid the content
	// out, trackBottom the footer's bottom as the last frame placed it,
	// and trackHeight the height last reported, all in DIPs.
	trackSig    string
	trackBottom float32
	trackHeight int
}

// ResetHeight forgets what the tracker measured, as the panel reopens in
// a fresh window whose first frames measure it anew.
func (p *Popover) ResetHeight() {
	p.trackSig, p.trackBottom, p.trackHeight = "", -1, 0
}

// View builds the panel.
func (p *Popover) View(c *ui.Context) {
	tk := applyTheme(c)
	m := p.Model
	snap := m.Timer()
	phaseMoved := snap.Phase != p.lastPhase
	if phaseMoved {
		p.lastPhase, p.phaseAt = snap.Phase, c.Now()
	}

	// What moves the layout: the work card's face, a feedback line under
	// the record button, the tag the goal reached adds.
	feedbackText, at := m.DrinkFeedback()
	feedbackOn := feedbackText != "" && c.Now().Sub(at) < FeedbackFlash
	sig := ""
	if snap.Phase != core.PhaseIdle {
		sig += "run"
	}
	if feedbackOn {
		sig += "feed"
	}
	if m.TodayCount() >= m.DailyGoal() {
		sig += "goal"
	}

	var footer ui.Element
	ui.Column(c).Fill().Padding(panelPad).Gap(10).Background(tk.BgContainer).Children(func() {
		p.header(c, m, tk).Key("pop-header").Transition(enter(0))

		Panel(c, "工作计时", func() {
			// The phase's state names itself beside the title, never by
			// color alone.
			Tag(c, snap.Phase.String(), phaseTag(snap.Phase))
		}, func() {
			p.workCard(c, m, snap, phaseMoved, tk)
		}).Key("pop-work").Transition(enter(40 * time.Millisecond))

		Panel(c, "喝水提醒", nil, func() {
			p.hydrationCard(c, m, tk)
		}).Key("pop-hydration").Transition(enter(80 * time.Millisecond))

		footer = p.footer(c, m, tk).Key("pop-footer").Transition(enter(120 * time.Millisecond))
	})
	p.trackContent(c, footer, sig, PanelHeight(snap.Phase, feedbackOn))
}

// trackContent keeps the panel's window as tall as its content, and
// moves it there smoothly: the window springs to the height the state
// asks for the moment the state moves — one curve at the frame rate, in
// step with the content's own animations, which chasing the laid-out
// bottom frame by frame cannot match — while the footer's measured box,
// as the previous frame laid it out, keeps the window from ever being
// left behind: never smaller than the content needs, or the footer
// would clip. A state that moved the layout asks for one more frame,
// whose reading then shows the move.
func (p *Popover) trackContent(c *ui.Context, footer ui.Element, sig string, target int) {
	if p.OnHeight == nil {
		return
	}
	window := ui.Animation{Spring: ui.SpringOptions{Stiffness: 300, Damping: 26, Mass: 1}}
	h := int(math.Ceil(float64(footer.AnimateTo("winh", float32(target), window))))
	if b := footer.Bounds(); b.H > 0 {
		if need := int(math.Ceil(float64(b.Y+b.H))) + panelPad; need > h {
			h = need
		}
		if sig != p.trackSig || math.Abs(float64(b.Y+b.H-p.trackBottom)) > 0.01 {
			p.trackSig, p.trackBottom = sig, b.Y+b.H
			c.After(0) // come back and measure the layout this frame leaves
		}
	}
	if h != p.trackHeight {
		p.trackHeight = h
		p.OnHeight(h)
	}
}

func phaseTag(phase core.Phase) display.TagColor {
	switch phase {
	case core.PhaseRunning:
		return display.TagSuccess
	case core.PhasePaused:
		return display.TagWarning
	}
	return ""
}

func (p *Popover) header(c *ui.Context, m PopoverModel, tk *theme.Tokens) ui.Element {
	return ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		Brand(c, 30)
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			display.Text(c, "Restify", display.TextProps{Strong: true, FontSize: 15})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				display.Hotkey(c, "mod+shift+t", display.HotkeyProps{Variant: display.HotkeyBorderless, Compact: true})
				display.Text(c, "计时", display.TextProps{Type: display.TextSecondary, FontSize: 11})
				display.Hotkey(c, "mod+shift+h", display.HotkeyProps{Variant: display.HotkeyBorderless, Compact: true})
				display.Text(c, "喝水", display.TextProps{Type: display.TextSecondary, FontSize: 11})
			})
		})
		if action.ActionIcon(c, iconSettings, action.ActionIconProps{
			Variant: action.ActionIconBorderless,
			Size:    theme.Middle,
			Title:   "设置",
		}).Clicked() {
			m.OpenSettings()
		}
	})
}

// workCard shows the timer compact while idle and opens up as it runs;
// the title stays put as the face below it changes.
func (p *Popover) workCard(c *ui.Context, m PopoverModel, snap core.Snapshot, phaseMoved bool, tk *theme.Tokens) {
	if snap.Phase == core.PhaseIdle {
		// Compact: what the next run is, and the button that starts it.
		ui.Column(c).Key("work-idle").Transition(swap).Gap(12).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1)
				s := m.Settings()
				display.Text(c, "工作 "+core.FormatShort(s.WorkDuration)+" · 休息 "+core.FormatShort(s.GracePeriod),
					display.TextProps{Type: display.TextSecondary, FontSize: 12}).SingleLine()
			})
			start := action.Button(c, "开始工作", action.ButtonProps{Variant: action.Primary, Size: theme.Middle})
			if start.Clicked() {
				m.ToggleTimer()
			}
		})
		return
	}

	// Full: the countdown is the hero.
	ui.Column(c).Key("work-run").Transition(swap).Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1)
			display.Text(c, "目标 "+core.FormatShort(snap.Total),
				display.TextProps{Type: display.TextSecondary, FontSize: 12}).
				FontFeatures("tnum").SingleLine()
		})

		cd := display.Text(c, core.FormatClock(snap.Remaining),
			display.TextProps{FontSize: 38, Strong: true}).FontFeatures("tnum")
		// A phase change springs the number a touch larger as it settles.
		if phaseMoved {
			cd.Jump("pulse", 1.06)
		}
		scale := cd.AnimateTo("pulse", 1.0, snappy)
		cd.Scale(scale, scale)

		if snap.Total > 0 {
			ProgressBar(c, 1-float64(snap.Remaining)/float64(snap.Total), AccentWork.Color(tk))
		}

		ui.Row(c).Gap(8).Children(func() {
			label, variant := "暂停", action.Primary
			if snap.Phase == core.PhasePaused {
				label, variant = "继续", action.Primary
			}
			toggle := action.Button(c, label, action.ButtonProps{Variant: variant, Size: theme.Middle})
			if toggle.Clicked() {
				m.ToggleTimer()
			}
			if action.Button(c, "重置", action.ButtonProps{Variant: action.Default, Size: theme.Middle}).Clicked() {
				m.ResetTimer()
			}
			if action.Button(c, "跳过休息", action.ButtonProps{Variant: action.Text, Size: theme.Middle}).Clicked() {
				m.SkipBreak()
			}
		})
	})
}

// hydrationCard shows today's drinks as a ring that fills, bouncing as
// one is recorded.
func (p *Popover) hydrationCard(c *ui.Context, m PopoverModel, tk *theme.Tokens) {
	today, goal := m.TodayCount(), m.DailyGoal()
	frac := 0.0
	if goal > 0 {
		frac = float64(today) / float64(goal)
	}

	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		// The ring bounces as a drink is recorded.
		ring := ui.Box(c)
		target := float32(1)
		if text, at := m.DrinkFeedback(); text != "" && c.Now().Sub(at) < 300*time.Millisecond {
			target = 1.08
			c.After(300*time.Millisecond - c.Now().Sub(at))
		}
		bounce := ring.AnimateTo("bounce", target, bouncy)
		ring.Scale(bounce, bounce)
		ring.Children(func() {
			feedback.Progress(c, float32(frac*100), feedback.ProgressProps{
				Type:     feedback.ProgressCircle,
				Size:     theme.Large,
				ShowInfo: true,
				Color:    AccentWater.Color(tk),
				Format:   percentLabel,
			})
		})

		if today >= goal {
			Tag(c, "已达标", display.TagSuccess)
		}

		ui.Column(c).Grow(1).Basis(0).Gap(3).Children(func() {
			display.Text(c, "今日饮水", display.TextProps{Type: display.TextSecondary, FontSize: 13})
			ui.Row(c).Gap(5).AlignItems(ui.End).Children(func() {
				display.Text(c, strconv.Itoa(today), display.TextProps{FontSize: 26, Strong: true}).
					TextColor(AccentWater.Color(tk)).FontFeatures("tnum")
				display.Text(c, "/ "+strconv.Itoa(goal)+" 杯", display.TextProps{Type: display.TextSecondary, FontSize: 13}).
					Padding(0, 0, 4, 0)
			})
		})
	})

	record := action.Button(c, "记录喝水", action.ButtonProps{Variant: action.Primary, Size: theme.Middle})
	if record.Clicked() {
		m.RecordDrink()
	}

	// The 3s feedback under the button, as the panel's banner.
	if text, at := m.DrinkFeedback(); text != "" && c.Now().Sub(at) < FeedbackFlash {
		display.Text(c, text, display.TextProps{Type: display.TextSecondary, FontSize: 12})
		if left := FeedbackFlash - c.Now().Sub(at); left > 0 {
			c.After(left)
		}
	}
}

func (p *Popover) footer(c *ui.Context, m PopoverModel, tk *theme.Tokens) ui.Element {
	return ui.Column(c).Gap(8).Children(func() {
		Hairline(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			stats := action.Button(c, "统计", action.ButtonProps{Variant: action.Text, Size: theme.Small})
			if stats.Clicked() {
				m.OpenStats()
			}
			ui.Box(c).Grow(1)
			// A found version takes the version string's place, as a
			// link that opens the update window.
			if v := m.UpdateAvailable(); v != "" {
				update := action.Button(c, "新版本 "+v, action.ButtonProps{Variant: action.Link, Size: theme.Small})
				if update.Clicked() {
					m.CheckForUpdates()
				}
			} else {
				display.Text(c, "Restify "+m.Version(), display.TextProps{Type: display.TextSecondary, FontSize: 11}).SingleLine()
			}
		})
	})
}
