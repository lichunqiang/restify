package gui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/motion"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

// The room inside a panel and between its parts, in DIPs.
const (
	panelPadX = 14
	panelPadY = 14
	panelGap  = 10
)

// enter slides a section in as the panel opens; the delays rise down the
// panel so the cards arrive one after another, on Lobe's snappy spring.
func enter(delay time.Duration) ui.ElementTransition {
	a := motion.Snappy(360 * time.Millisecond)
	a.Delay = delay
	return ui.ElementTransition{Animation: &a, Enter: &ui.Motion{Y: 8}}
}

// popIn springs an icon in from nothing as its window opens: the icon's
// scale channel starts at 0 and grows on Lobe's bouncy spring, which
// waits out the delay itself, so the icons of a page say hello one after
// another. Unlike a Transition's Enter — which never runs on a whole
// new window, born all in one frame — this plays on every open. The
// element's place among its siblings must stay put across frames.
func popIn(el ui.Element, delay time.Duration) {
	if first := ui.Local(el, "pop", func() bool { return false }); !*first {
		*first = true
		el.Jump("pop", 0)
	}
	a := motion.Bouncy(520 * time.Millisecond)
	a.Delay = delay
	s := el.AnimateTo("pop", 1, a)
	el.Scale(s, s)
}

// swap collapses an element away and opens the one that replaces it, as
// the work card changes faces.
var swap = motion.Collapse()

// The springs the interface moves with, from Lobe's motion presets.
var (
	snappy = motion.Snappy(360 * time.Millisecond)
	bouncy = motion.Bouncy(520 * time.Millisecond)
)

// Panel is a filled block: the card a page's section sits on, headed by its
// title and the actions that belong to it. body draws the content under the
// heading.
func Panel(c *ui.Context, title string, actions, body func()) ui.Element {
	b := layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(panelPadX, panelPadY).Gap(panelGap)
	b.Children(func() {
		if title != "" || actions != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if title != "" {
					display.Text(c, title, display.TextProps{Strong: true, FontSize: 15})
				}
				ui.Box(c).Grow(1)
				if actions != nil {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(actions)
				}
			})
		}
		if body != nil {
			body()
		}
	})
	return b
}

// Card is a filled block without a heading.
func Card(c *ui.Context, body func()) ui.Element {
	b := layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(panelPadX, panelPadY).Gap(panelGap)
	b.Children(func() {
		if body != nil {
			body()
		}
	})
	return b
}

// SettingsCard groups settings rows on a filled block under a title.
func SettingsCard(c *ui.Context, title string, rows func()) ui.Element {
	col := ui.Column(c).Gap(8)
	col.Children(func() {
		if title != "" {
			SectionTitle(c, title)
		}
		layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(4, 12).Children(func() {
			if rows != nil {
				rows()
			}
		})
	})
	return col
}

// SectionTitle heads a group of a page.
func SectionTitle(c *ui.Context, title string) {
	display.Text(c, title, display.TextProps{Type: display.TextSecondary, Strong: true, FontSize: 13}).Padding(0, 4)
}

// SettingRow is a row of a settings card: a label with a detail line at the
// left, the control at the right.
func SettingRow(c *ui.Context, label, detail string, control func()) {
	ui.Row(c).Gap(16).Padding(10, 0).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			display.Text(c, label, display.TextProps{})
			if detail != "" {
				display.Text(c, detail, display.TextProps{Type: display.TextSecondary, FontSize: 12}).MaxLines(2)
			}
		})
		if control != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(control)
		}
	})
}

// RowSeparator draws the line between two settings rows.
func RowSeparator(c *ui.Context) {
	layout.Divider(c, layout.DividerProps{}).MarginY(2)
}

// Hairline is the line between two parts of a card.
func Hairline(c *ui.Context) ui.Element {
	return layout.Divider(c, layout.DividerProps{})
}

// Notice shows an informative message, as Lobe's soft info alert.
func Notice(c *ui.Context, text string) {
	feedback.Alert(c, text, feedback.AlertProps{Type: feedback.Info, Variant: feedback.Soft})
}

// ErrorBanner shows an error; nothing for nil.
func ErrorBanner(c *ui.Context, err error) {
	if err == nil {
		return
	}
	feedback.Alert(c, err.Error(), feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
}

// EmptyState fills a page or list with a message and, optionally, the one
// action that starts it.
func EmptyState(c *ui.Context, icon *ui.SVG, title, detail string, action func()) {
	display.Empty(c, display.EmptyProps{Icon: icon, Title: title, Description: detail, Action: action, Page: true})
}

// BigValue is a caption over a large figure and its unit.
func BigValue(c *ui.Context, caption, value, unit string, col ui.Color) ui.Element {
	col0 := ui.Column(c).Gap(2)
	col0.Children(func() {
		if caption != "" {
			display.Text(c, caption, display.TextProps{Type: display.TextSecondary, FontSize: 12})
		}
		ui.Row(c).Gap(5).AlignItems(ui.End).Children(func() {
			display.Text(c, value, display.TextProps{FontSize: 34, Strong: true}).TextColor(col).FontFeatures("tnum")
			if unit != "" {
				display.Text(c, unit, display.TextProps{Type: display.TextSecondary, FontSize: 14}).
					Padding(0, 0, 5, 0)
			}
		})
	})
	return col0
}

// Metric is a labeled figure of a Metrics row.
type Metric struct {
	Label, Value string
	// Color colors the value; zero leaves it the text color.
	Color ui.Color
}

// Metrics lays labeled figures out in equal columns.
func Metrics(c *ui.Context, ms ...Metric) {
	ui.Row(c).Gap(12).Children(func() {
		for _, m := range ms {
			ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
				display.Text(c, m.Label, display.TextProps{Type: display.TextSecondary, FontSize: 12})
				col := m.Color
				if col == (ui.Color{}) {
					col = c.Theme().Text
				}
				display.Text(c, m.Value, display.TextProps{Strong: true, FontSize: 14}).TextColor(col).FontFeatures("tnum")
			})
		}
	})
}

// KV is a labeled value in a card.
func KV(c *ui.Context, label, value string) {
	ui.Row(c).Gap(12).Children(func() {
		display.Text(c, label, display.TextProps{Type: display.TextSecondary, FontSize: 12}).Width(110)
		display.Text(c, value, display.TextProps{FontSize: 12}).Grow(1).Basis(0).Selectable()
	})
}

// percentLabel writes a progress percent as whole percent: "42%". Lobe's
// default formatter writes the raw float, which a 5/12 ring renders as
// "41.66666%", too wide for the circle.
func percentLabel(p float32) string {
	return strconv.Itoa(int(math.Round(float64(p)))) + "%"
}

// ProgressBar is the thin line of Lobe's progress, filled to frac.
func ProgressBar(c *ui.Context, frac float64, col ui.Color) ui.Element {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return feedback.Progress(c, float32(frac*100), feedback.ProgressProps{
		Type:  feedback.ProgressLine,
		Size:  theme.Middle,
		Color: col,
	})
}

// Tag is a status in a Lobe tag: the capsule that names a state.
func Tag(c *ui.Context, label string, col display.TagColor) ui.Element {
	return display.Tag(c, label, display.TagProps{
		Variant: display.TagFilled,
		Color:   col,
		Round:   true,
		Size:    theme.Small,
	})
}

// DragStrip is the strip at a page's top where its title used to be: it
// clears the title bar and drags the window, taking no more room than
// the traffic lights need, while the page's own navigation names it.
func DragStrip(c *ui.Context) {
	bar := c.TitleBar()
	ui.Box(c).Height(max(bar.Height, 28)).DragWindow()
}

// Accent names the palette color that marks a part of the app: the timer,
// the water, the charts, the rest. Colors come from Lobe's tokens, so they
// follow the light and dark themes.
type Accent int

const (
	AccentWork Accent = iota
	AccentWater
	AccentChart
	AccentRest
	AccentWarn
)

// Color is the accent's color.
func (a Accent) Color(tk *theme.Tokens) ui.Color {
	switch a {
	case AccentWater:
		return tk.Info
	case AccentChart:
		return tk.Success
	case AccentRest:
		return tk.Purple
	case AccentWarn:
		return tk.Warning
	}
	return tk.Primary
}

// Fill is the accent as a soft wash, for a mark's background.
func (a Accent) Fill(tk *theme.Tokens) ui.Color {
	return a.Color(tk).Alpha(0.14)
}

// Tag is the accent as the palette name of a Lobe tag.
func (a Accent) Tag() display.TagColor {
	switch a {
	case AccentWater:
		return display.TagInfo
	case AccentChart:
		return display.TagSuccess
	case AccentRest:
		return display.TagPurple
	case AccentWarn:
		return display.TagWarning
	}
	return ""
}

// Brand is the app's mark: the droplet on a soft wash of the water accent.
func Brand(c *ui.Context, size float32) ui.Element {
	tk := theme.TokensOf(c)
	icon := display.IconMiddle
	if size >= 38 {
		icon = display.IconLarge
	}
	return ui.Row(c).Size(size, size).Radius(size * 0.32).Background(AccentWater.Fill(tk)).
		Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		display.Icon(c, lucide.Droplet(), display.IconProps{Size: icon}).TextColor(AccentWater.Color(tk))
	})
}
