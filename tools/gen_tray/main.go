// gen_tray renders the tray glyphs to the template PNGs the app embeds:
// 16pt icons at 2x (32px), black on transparent, one per icon style and
// state. Run from app/: go run ./tools/gen_tray
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"restify/internal/gui"
	"restify/internal/store"
)

const (
	size  = 16 // DIPs
	scale = 2  // 32px on Retina
)

func render(icon *ui.SVG) []byte {
	view := func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Column(c).Fill().Children(func() {
			ui.Icon(c, icon).Size(size, size).TextColor(ui.RGB(0, 0, 0))
		})
	}
	tt := ui.NewTester(view, size, size)
	tt.SetScale(scale)
	var buf buffer
	if err := png.Encode(&buf, tt.Image()); err != nil {
		panic(err)
	}
	return buf.bytes
}

type buffer struct{ bytes []byte }

func (b *buffer) Write(p []byte) (int, error) {
	b.bytes = append(b.bytes, p...)
	return len(p), nil
}

func main() {
	outDir := filepath.Join("internal", "app", "trayicons")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	styles := []store.IconStyle{store.IconTimer, store.IconMoon, store.IconClock, store.IconHourglass, store.IconDrop}
	for _, style := range styles {
		for _, running := range []bool{false, true} {
			state := "idle"
			if running {
				state = "running"
			}
			name := filepath.Join(outDir, string(style)+"-"+state+".png")
			if err := os.WriteFile(name, render(gui.Icon(style, running)), 0o644); err != nil {
				panic(err)
			}
			fmt.Println("wrote", name)
		}
	}
}
