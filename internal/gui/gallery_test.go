package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

// gallery shows the design's vocabulary on one page: the Lobe MyGo
// components the views are built from, in the window theme and over the
// popover material.
func gallery(c *ui.Context) {
	tk := applyTheme(c)
	text := "添加一条文案…"
	volume := 0.6
	on := true
	choiceSel := 1

	ui.Column(c).Fill().Padding(24).Gap(14).Background(tk.BgLayout).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			Brand(c, 34)
			ui.Column(c).Gap(2).Children(func() {
				display.Text(c, "Restify 组件", display.TextProps{Heading: 4})
				display.Text(c, "Lobe MyGo · Lobe UI tokens", display.TextProps{Type: display.TextSecondary, FontSize: 12})
			})
			ui.Box(c).Grow(1)
			display.Hotkey(c, "mod+shift+t", display.HotkeyProps{Variant: display.HotkeyFilled})
		})

		Panel(c, "工作计时", func() { Tag(c, "运行中", display.TagSuccess) }, func() {
			BigValue(c, "剩余时间", "18:32", "", AccentWork.Color(tk))
			ProgressBar(c, 0.62, AccentWork.Color(tk))
			Metrics(c,
				Metric{Label: "今日完成", Value: "3"},
				Metric{Label: "喝水", Value: "5/8"},
				Metric{Label: "连续达标", Value: "4 天", Color: AccentChart.Color(tk)},
			)
		})

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			Tag(c, "进行中", display.TagSuccess)
			Tag(c, "已暂停", display.TagWarning)
			Tag(c, "未开始", "")
			Tag(c, "已达标", display.TagProcessing)
		})

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			action.Button(c, "开始", action.ButtonProps{Variant: action.Primary})
			action.Button(c, "重置", action.ButtonProps{Variant: action.Default})
			action.Button(c, "跳过休息", action.ButtonProps{Variant: action.Text})
			ui.Box(c).Grow(1)
			action.ActionIcon(c, iconPlay, action.ActionIconProps{Variant: action.ActionIconFilled, Size: theme.Middle, Title: "开始"})
			action.ActionIcon(c, iconRefresh, action.ActionIconProps{Variant: action.ActionIconBorderless, Size: theme.Middle, Title: "刷新"})
			action.ActionIcon(c, iconMoon, action.ActionIconProps{Variant: action.ActionIconBorderless, Size: theme.Middle, Active: true, Title: "深色"})
		})

		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			action.Segmented(c, &choiceSel, []string{"15", "25", "45", "60"}, action.SegmentedProps{Size: theme.Middle})
			ui.Box(c).Grow(1)
			choice.Switch(c, &on, choice.SwitchProps{Size: theme.Middle})
		})

		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(150).Children(func() {
				choice.Slider(c, &volume, choice.SliderProps{Min: 0, Max: 1, Step: 0.05})
			})
			ui.Box(c).Width(180).Children(func() {
				field.Input(c, &text, field.InputProps{Placeholder: "添加一条文案…", Size: theme.Middle})
			})
		})

		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			feedback.Progress(c, 37, feedback.ProgressProps{
				Type: feedback.ProgressCircle, Size: theme.Large, ShowInfo: true, Color: AccentWater.Color(tk),
				Format: percentLabel,
			})
			feedback.Progress(c, 72, feedback.ProgressProps{
				Type: feedback.ProgressCircle, Size: theme.Middle, ShowInfo: true, Status: feedback.ProgressSuccess,
				Format: percentLabel,
			})
			ui.Box(c).Grow(1).Children(func() {
				feedback.Progress(c, 72, feedback.ProgressProps{Type: feedback.ProgressLine, Size: theme.Middle, ShowInfo: true})
			})
		})

		Notice(c, "成年人每天应该喝 1.5-2 升水。")
		ErrorBanner(c, errExample{})
		EmptyState(c, iconDrops, "还没有记录", "记录一次喝水，开始今天的进度。", func() {
			action.Button(c, "记录喝水", action.ButtonProps{Variant: action.Primary, Size: theme.Middle})
		})

		Card(c, func() {
			KV(c, "版本", "1.0.0")
			Hairline(c)
			KV(c, "图标", "Lucide")
			Hairline(c)
			RowSeparator(c)
		})
	})
}

// errExample is a stand-in error for the gallery's banner.
type errExample struct{}

func (errExample) Error() string { return "导出失败：没有写入权限" }

func TestGallery(t *testing.T) {
	for _, tc := range []struct {
		name           string
		dark, vibrancy bool
	}{
		{"light", false, false},
		{"dark", true, false},
		{"material-light", false, true},
		{"material-dark", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := ui.NewTester(gallery, 480, 980)
			tt.SetDark(tc.dark)
			tt.SetVibrancy(tc.vibrancy)
			// The components transition their colors in; let them settle
			// before the appearance is read and captured.
			tt.Advance(400 * time.Millisecond)
			for _, want := range []string{"Restify 组件", "工作计时", "18:32", "进行中", "记录喝水", "成年人每天应该喝 1.5-2 升水。"} {
				if !tt.HasText(want) {
					t.Errorf("%s: %q missing from texts %q", tc.name, want, tt.Texts())
				}
			}
			for _, want := range []string{"导出失败：没有写入权限"} {
				if !tt.HasText(want) {
					t.Errorf("%s: %q missing from texts %q", tc.name, want, tt.Texts())
				}
			}
			if dir := os.Getenv("RESTIFY_GALLERY_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, "gallery-"+tc.name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
