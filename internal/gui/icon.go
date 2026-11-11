package gui

import (
	"github.com/egoist/mygo/ui"

	"restify/internal/store"
)

// Icon returns the glyph for a menu-bar icon style, filled when the
// timer runs. The tray renders it as a template image.
func Icon(style store.IconStyle, running bool) *ui.SVG {
	type glyph struct{ idle, run *ui.SVG }
	set := map[store.IconStyle]glyph{
		store.IconTimer:     {icTimer, icTimerFill},
		store.IconMoon:      {icMoon, icMoonFill},
		store.IconClock:     {icClock, icClockFill},
		store.IconHourglass: {icHourglass, icHourglassFill},
		store.IconDrop:      {icDrop, icDropFill},
	}[style]
	if set.idle == nil {
		set = glyph{icTimer, icTimerFill}
	}
	if running {
		return set.run
	}
	return set.idle
}
