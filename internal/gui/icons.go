package gui

import (
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

// The icons the interface draws, from Lobe MyGo's Lucide set: 24 DIP
// glyphs that take the color of the text beside them.
//
// The menu-bar images are separate: they are generated templates from
// Remix Icon (see glyphs_gen.go and tools/gen_tray), which system chrome
// wants as masks.
var (
	iconTimer     = lucide.Timer()
	iconDrop      = lucide.Droplet()
	iconDrops     = lucide.Droplets()
	iconMoon      = lucide.Moon()
	iconClock     = lucide.Clock()
	iconHourglass = lucide.Hourglass()
	iconSettings  = lucide.Settings()
	iconChart     = lucide.ChartLine()
	iconCalendar  = lucide.Calendar()
	iconTrophy    = lucide.Trophy()
	iconFlame     = lucide.Flame()
	iconMedal     = lucide.Medal()
	iconBell      = lucide.Bell()
	iconPlay      = lucide.Play()
	iconPause     = lucide.Pause()
	iconReset     = lucide.RotateCcw()
	iconSkip      = lucide.SkipForward()
	iconPlus      = lucide.Plus()
	iconClose     = lucide.X()
	iconTrash     = lucide.Trash2()
	iconExport    = lucide.Download()
	iconImport    = lucide.Upload()
	iconVolume    = lucide.Volume2()
	iconAlert     = lucide.TriangleAlert()
	iconInfo      = lucide.Info()
	iconCheck     = lucide.Check()
	iconRefresh   = lucide.RefreshCw()
	iconSparkles  = lucide.Sparkles()
	iconCoffee    = lucide.Coffee()
)
