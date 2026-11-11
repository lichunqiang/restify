package app

import (
	"errors"
	"log"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/mp3"
	_ "github.com/egoist/mygo/audio/vorbis"
)

// notificationsSupported is mygo.NotificationsSupported, split so
// headless tests can answer it: without App.Run the real one waits on
// the main thread forever.
var notificationsSupported = mygo.NotificationsSupported

// notify sends a desktop notification with the given id (replacing one
// still shown with the same id) and routes its click.
func (a *App) notify(id, title, body string, onClick func()) {
	a.notifyResult(id, title, body, onClick, nil)
}

// notifyResult also reports through done how showing went: nil once the
// system has the notification, errors.ErrUnsupported when this run
// cannot show any (an unpackaged build on macOS). done may run on the
// main thread or the showing goroutine.
func (a *App) notifyResult(id, title, body string, onClick func(), done func(error)) {
	if !notificationsSupported() {
		if done != nil {
			done(errors.ErrUnsupported)
		}
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  body,
		Group: "restify",
	})
	if onClick != nil {
		click := onClick
		n.OnClick(func() {
			click()
			n.Close()
		})
	}
	// Show waits for the user's first permission answer: never on the
	// main thread.
	go func() {
		err := n.Show()
		if err != nil {
			log.Println("notification:", err)
		}
		if done != nil {
			done(err)
		}
	}()
}

// notifySimple is a notification without a click action.
func (a *App) notifySimple(title, body string) {
	a.notify("", title, body, nil)
}

// notificationResultNotice words how a test notification went, for the
// settings row to show instead of failing silently.
func notificationResultNotice(err error) string {
	switch {
	case err == nil:
		return "测试通知已发送，请查看通知中心"
	case errors.Is(err, mygo.ErrNotificationsDenied):
		return "通知被拒绝：请在 系统设置 → 通知 → Restify 中允许通知"
	case errors.Is(err, errors.ErrUnsupported):
		return "此运行方式不支持系统通知，请用 mygo dev 或 mygo build 打包运行"
	default:
		return "通知发送失败：" + err.Error()
	}
}

// soundScale is each event's share of the master volume, as the Swift
// app scaled them.
func soundScale(kind string) float64 {
	switch kind {
	case "hydration":
		return 0.8
	case "success":
		return 0.6
	default:
		return 1.0
	}
}

// soundPlayer is what the app needs from a sound it plays: the part of
// mygo's *audio.Player it uses. It is an interface so headless tests can
// record playback instead of opening an audio device.
type soundPlayer interface {
	Play()  // from the current position, or from the start after the end
	Pause() // where it is
	Stop()  // back to the start
	Playing() bool
	// OnEnd calls fn on a goroutine of its own when the sound plays to
	// its end; the returned func removes the listener.
	OnEnd(fn func()) func()
	Close()
}

// mygoSoundPlayer adapts *audio.Player to soundPlayer.
type mygoSoundPlayer struct{ p *audio.Player }

func (p mygoSoundPlayer) Play()                  { p.p.Play() }
func (p mygoSoundPlayer) Pause()                 { p.p.Pause() }
func (p mygoSoundPlayer) Stop()                  { p.p.Stop() }
func (p mygoSoundPlayer) Playing() bool          { return p.p.Playing() }
func (p mygoSoundPlayer) OnEnd(fn func()) func() { return p.p.OnEnd(fn) }
func (p mygoSoundPlayer) Close()                 { p.p.Close() }

// newSoundPlayer builds a paused player of s at the given volume. It is
// a variable so tests can inject one that makes no sound.
var newSoundPlayer = func(s *audio.Sound, volume float64) soundPlayer {
	return mygoSoundPlayer{s.NewPlayer().SetVolume(volume)}
}

// soundVoice is one player the app manages: a reminder sound's, or the
// settings' audition.
type soundVoice struct {
	kind   string
	player soundPlayer
	paused bool // the audition waits paused; event sounds never do
}

// soundCache holds a decoded *audio.Sound per path.
var soundCache sync.Map // path -> *audio.Sound

// soundForPath returns path's decoded sound, from the cache when it was
// decoded before: reminders repeat, so each path decodes once.
func soundForPath(path string) (*audio.Sound, error) {
	if cached, ok := soundCache.Load(path); ok {
		if snd, ok := cached.(*audio.Sound); ok {
			return snd, nil
		}
	}
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	snd, err := audio.DecodeSound(data)
	if err != nil {
		return nil, err
	}
	soundCache.Store(path, snd)
	return snd, nil
}

// playSound plays the custom sound for kind, or the system beep when
// none is set (Restify ships no sound files, as its Swift self didn't).
// A new sound of the same kind replaces the one still playing: two
// reminders must not play over each other.
func (a *App) playSound(kind string, scale float64) {
	s := a.set.Get()
	if !s.EnableSound {
		return
	}
	path := s.CustomHydrationSoundPath
	switch kind {
	case "break":
		path = s.CustomBreakSoundPath
	case "success":
		path = "" // the success chime has no custom slot
	}
	if path == "" {
		mygo.Shell.Beep()
		return
	}

	snd, err := soundForPath(path)
	if err != nil {
		log.Println("sound:", filepath.Base(path), err)
		mygo.Shell.Beep()
		return
	}
	p := newSoundPlayer(snd, s.SoundVolume*scale)

	a.soundMu.Lock()
	if a.sounds == nil {
		a.sounds = map[string]*soundVoice{}
	}
	old := a.sounds[kind]
	a.sounds[kind] = &soundVoice{kind: kind, player: p}
	a.soundMu.Unlock()
	if old != nil {
		old.player.Stop()
		old.player.Close()
	}
	p.Play()
}

// TestSound auditions kind's reminder sound from the settings window. A
// click while it plays pauses it, a click while it waits paused resumes
// it, and a click once it ended starts it again from the beginning. One
// audition plays at a time, whichever row clicks, and the audition stays
// out of the reminder sounds' way.
func (a *App) TestSound(kind string) {
	if kind != "break" && kind != "hydration" {
		return // only the two rows have custom sounds to audition
	}
	s := a.set.Get()
	if !s.EnableSound {
		return
	}
	if a.toggleSoundPreview(kind) {
		a.tickUI()
		return
	}
	path := s.CustomBreakSoundPath
	if kind == "hydration" {
		path = s.CustomHydrationSoundPath
	}
	if path == "" {
		// The default is the system's own beep: there is nothing to pause.
		if a.stopSoundPreview() {
			a.tickUI()
		}
		mygo.Shell.Beep()
		return
	}
	snd, err := soundForPath(path)
	if err != nil {
		log.Println("sound:", filepath.Base(path), err)
		if a.stopSoundPreview() {
			a.tickUI()
		}
		mygo.Shell.Beep()
		return
	}
	a.startSoundPreview(kind, snd, s.SoundVolume)
	a.tickUI()
}

// SoundPreviewState reports whether kind's audition plays or waits
// paused, for its settings row to label the button.
func (a *App) SoundPreviewState(kind string) (playing, paused bool) {
	a.soundMu.Lock()
	defer a.soundMu.Unlock()
	v := a.preview
	if v == nil || v.kind != kind {
		return false, false
	}
	return v.player.Playing() && !v.paused, v.paused
}

// toggleSoundPreview pauses or resumes kind's audition, reporting false
// when no audition of kind plays or waits paused, so the caller starts
// one from the beginning.
func (a *App) toggleSoundPreview(kind string) bool {
	a.soundMu.Lock()
	v := a.preview
	if v == nil || v.kind != kind {
		a.soundMu.Unlock()
		return false
	}
	if v.paused {
		v.paused = false
		a.soundMu.Unlock()
		v.player.Play() // resumes where it paused
		return true
	}
	if !v.player.Playing() { // it ended: start over
		a.soundMu.Unlock()
		return false
	}
	v.paused = true
	a.soundMu.Unlock()
	v.player.Pause()
	return true
}

// startSoundPreview makes kind's sound the one audition, replacing any
// other audition (or one that ended), and reports its end to the windows.
func (a *App) startSoundPreview(kind string, snd *audio.Sound, volume float64) {
	p := newSoundPlayer(snd, volume)
	v := &soundVoice{kind: kind, player: p}

	a.soundMu.Lock()
	old := a.preview
	a.preview = v
	a.soundMu.Unlock()
	if old != nil {
		old.player.Stop()
		old.player.Close()
	}

	p.OnEnd(func() {
		// The sound ran out: the button says 试听 again. A callback from
		// a player replaced since must not clear the new one.
		a.soundMu.Lock()
		ended := a.preview == v
		if ended {
			a.preview = nil
		}
		a.soundMu.Unlock()
		if ended {
			a.tickUI()
		}
	})
	p.Play()
}

// stopSoundPreview stops and frees the audition, reporting whether one
// was playing or waiting paused.
func (a *App) stopSoundPreview() bool {
	a.soundMu.Lock()
	v := a.preview
	a.preview = nil
	a.soundMu.Unlock()
	if v == nil {
		return false
	}
	v.player.Stop()
	v.player.Close()
	return true
}

// stopRestSounds silences the break reminder and the audition when the
// rest ends: that audio belongs to the rest, not to what follows it. A
// drink's chime keeps playing.
func (a *App) stopRestSounds() {
	a.soundMu.Lock()
	event := a.sounds["break"]
	if event != nil {
		delete(a.sounds, "break")
	}
	preview := a.preview
	a.preview = nil
	a.soundMu.Unlock()
	for _, v := range []*soundVoice{event, preview} {
		if v != nil {
			v.player.Stop()
			v.player.Close()
		}
	}
}
