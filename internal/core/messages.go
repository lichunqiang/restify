// Package core runs Restify's reminders: the work timer with its idle
// detection and smart nudges, the hydration loop, and the forced-rest
// session. It knows no UI; the app wires its hooks to windows and
// notifications.
package core

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// motivationalBreakMessages are the built-in pep variants of the break
// reminder, chosen when EnableMotivationalMessages is on.
var motivationalBreakMessages = []string{
	"太棒了！您已经专注工作了 {{duration}}，现在该休息一下了！",
	"高效工作 {{duration}}，休息是为了走更远的路！",
	"完成了 {{duration}} 的工作，您做得很好！",
	"专注力满分！工作了 {{duration}}，奖励自己一个休息吧！",
}

// FormatShort renders a duration the way the Swift app did: "25分钟",
// "1小时30分钟".
func FormatShort(d time.Duration) string {
	mins := int(d / time.Minute)
	if mins >= 60 {
		return fmt.Sprintf("%d小时%d分钟", mins/60, mins%60)
	}
	return fmt.Sprintf("%d分钟", mins)
}

// FormatClock renders m:ss (or h:mm:ss), as the countdowns show it.
func FormatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d / time.Second)
	if secs/3600 > 0 {
		return fmt.Sprintf("%d:%02d:%02d", secs/3600, secs/60%60, secs%60)
	}
	return fmt.Sprintf("%02d:%02d", secs/60, secs%60)
}

// PickBreakMessage chooses the break reminder's body: a pep variant when
// motivational is on, else a random entry from the user's library.
func PickBreakMessage(library []string, motivational bool, workDuration time.Duration) string {
	var msg string
	if motivational {
		msg = motivationalBreakMessages[rand.Intn(len(motivationalBreakMessages))]
	} else if len(library) > 0 {
		msg = library[rand.Intn(len(library))]
	} else {
		msg = "休息时间到了！您已经工作了 {{duration}}，该休息一下了。"
	}
	return strings.ReplaceAll(msg, "{{duration}}", FormatShort(workDuration))
}

// PickHydrationMessage chooses the hydration reminder's body from the
// user's library.
func PickHydrationMessage(library []string) string {
	if len(library) == 0 {
		return "该喝水了！保持充足的水分对健康很重要。"
	}
	return library[rand.Intn(len(library))]
}

// HydrationBodyWithStats appends the day's progress to a chosen message,
// as the notification body showed it.
func HydrationBodyWithStats(msg string, count, goal int) string {
	progress := fmt.Sprintf("进度 %d/%d", count, goal)
	if count >= goal {
		progress = "已完成目标"
	}
	return fmt.Sprintf("%s（%s）", msg, progress)
}

// HydrationMessageWithStats picks a message and appends the day's
// progress.
func HydrationMessageWithStats(library []string, count, goal int) string {
	return HydrationBodyWithStats(PickHydrationMessage(library), count, goal)
}
