// gen_icon renders the app icon candidates as SVG and rasterizes them
// headlessly at 1024x1024. Run from the repo root:
//
//	go run ./tools/gen_icon            # all candidates to icon-candidates/
//	go run ./tools/gen_icon -o resources/icon.png quiet-ring
//
// The macOS shape is the squircle: an 824pt rounded square (radius 22.4%)
// centered in the 1024 canvas with transparent margins.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// squircle is the shared background path.
const squircle = `<rect x="100" y="100" width="824" height="824" rx="185" ry="185" fill="%s"/>`

func withGradient(id, from, to string) string {
	return `<defs><linearGradient id="` + id + `" x1="0" y1="0" x2="0" y2="1">` +
		`<stop offset="0" stop-color="` + from + `"/><stop offset="1" stop-color="` + to + `"/></linearGradient></defs>`
}

// drop is a water drop path of radius r centered at (cx, cy).
func drop(cx, cy, r float64) string {
	// The drop: a circle with a soft point at the top, ~1.35r tall, its
	// left and right sides mirrored curves.
	return fmt.Sprintf(
		`M %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f Z`,
		cx, cy-r*1.32, // top point
		cx-r*0.62, cy-r*0.62, cx-r, cy-r*0.10, cx-r, cy+r*0.28, // left side to bottom start
		cx-r, cy+r*0.72, cx-r*0.72, cy+r, cx, cy+r, // bottom left arc
		cx+r*0.72, cy+r, cx+r, cy+r*0.72, cx+r, cy+r*0.28, // bottom right arc
		cx+r, cy-r*0.10, cx+r*0.62, cy-r*0.62, cx, cy-r*1.32, // right side back up
	)
}

// designs are the candidates, in presentation order.
var designs = []struct {
	id   string
	name string
	svg  string
}{
	{
		id:   "quiet-ring",
		name: "静默圆环",
		svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">` +
			withGradient("bg", "#F7F7FA", "#EBEBF2") +
			fmt.Sprintf(squircle, "url(#bg)") +
			// The quiet track and a blue progress arc with a gap at 12.
			// The track sits a step darker than the surface so the ring
			// holds up at 32px.
			`<circle cx="512" cy="512" r="248" fill="none" stroke="#C9C9D2" stroke-width="84"/>` +
			`<path d="M 672.7 351.3 A 248 248 0 1 1 351.3 351.3" fill="none" stroke="#007AFF" stroke-width="84" stroke-linecap="round"/>` +
			// The drop sits in the gap, at twelve.
			`<path d="` + drop(512, 264, 58) + `" fill="#14A3BE"/>` +
			`</svg>`,
	},
	{
		id:   "pause-drops",
		name: "双滴暂停",
		svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">` +
			withGradient("bg", "#0A84FF", "#0057D8") +
			fmt.Sprintf(squircle, "url(#bg)") +
			// The pause: two rounded bars, the rest every work period ends in.
			`<rect x="392" y="356" width="92" height="312" rx="46" fill="#FFFFFF"/>` +
			`<rect x="540" y="356" width="92" height="312" rx="46" fill="#FFFFFF"/>` +
			// A drop under the pause, the other thing Restify keeps time on.
			`<path d="` + drop(512, 742, 40) + `" fill="#FFFFFF" opacity="0.9"/>` +
			`</svg>`,
	},
	{
		id:   "moon-drop",
		name: "月抱水滴",
		svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">` +
			withGradient("bg", "#32324A", "#1B1B2C") +
			fmt.Sprintf(squircle, "url(#bg)") +
			// A crescent: a circle with a bite taken by the sky.
			`<defs><mask id="crescent"><rect width="1024" height="1024" fill="#000"/>` +
			`<circle cx="492" cy="482" r="212" fill="#fff"/>` +
			`<circle cx="596" cy="420" r="196" fill="#000"/></mask></defs>` +
			`<rect x="216" y="206" width="552" height="552" fill="#F5F5F7" mask="url(#crescent)"/>` +
			// The drop rests in the crescent's hollow.
			`<path d="` + drop(636, 632, 66) + `" fill="#40C8E0"/>` +
			`</svg>`,
	},
	{
		id:   "hourglass-drop",
		name: "沙漏水滴",
		svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">` +
			withGradient("bg", "#1CA8C2", "#0E7A92") +
			fmt.Sprintf(squircle, "url(#bg)") +
			// The hourglass, drawn as one stroked path with soft joins.
			`<path d="M 384 336 H 640 V 380 L 546 486 Q 512 512 546 538 L 640 644 V 688 H 384 V 644 L 478 538 Q 512 512 478 486 L 384 380 Z"
				fill="none" stroke="#FFFFFF" stroke-width="42" stroke-linejoin="round" stroke-linecap="round"/>` +
			// Falling: a drop just below the waist.
			`<path d="` + drop(512, 586, 34) + `" fill="#FFFFFF"/>` +
			`</svg>`,
	},
}

func render(svg string) []byte {
	parsed := ui.MustParseSVG([]byte(svg))
	view := func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Image(c, parsed).Fill()
	}
	tt := ui.NewTester(view, 1024, 1024)
	var buf bufWriter
	if err := png.Encode(&buf, tt.Image()); err != nil {
		panic(err)
	}
	return buf.b
}

type bufWriter struct{ b []byte }

func (w *bufWriter) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }

func main() {
	out := flag.String("o", "icon-candidates", "output directory, or a file with one design id")
	flag.Parse()
	only := flag.Arg(0)

	isFile := false
	if flag.NArg() > 0 && filepath.Ext(*out) != "" {
		isFile = true
	}
	if !isFile {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			panic(err)
		}
	}
	for _, d := range designs {
		if only != "" && d.id != only {
			continue
		}
		path := *out
		if !isFile {
			path = filepath.Join(*out, d.id+".png")
		}
		if err := os.WriteFile(path, render(d.svg), 0o644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %s (%s)\n", path, d.name)
	}
}
