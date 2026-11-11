package app

import (
	"sync"
	"testing"

	"github.com/egoist/mygo/audio"

	"restify/internal/store"
)

// ---- fakes ----

// fakeSoundPlayer stands in for a playing sound in tests, where no audio
// device exists: it records what the app asked of it, and lets a test run
// the end listener the real player would run at the sound's end.
type fakeSoundPlayer struct {
	mu     sync.Mutex
	volume float64
	played int
	pauses int
	stops  int
	closes int
	on     bool
	onEnd  []func()
}

func (p *fakeSoundPlayer) Play() {
	p.mu.Lock()
	p.played++
	p.on = true
	p.mu.Unlock()
}

func (p *fakeSoundPlayer) Pause() {
	p.mu.Lock()
	p.pauses++
	p.on = false
	p.mu.Unlock()
}

func (p *fakeSoundPlayer) Stop() {
	p.mu.Lock()
	p.stops++
	p.on = false
	p.mu.Unlock()
}

func (p *fakeSoundPlayer) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.on
}

func (p *fakeSoundPlayer) OnEnd(fn func()) func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEnd = append(p.onEnd, fn)
	return func() {}
}

func (p *fakeSoundPlayer) Close() {
	p.mu.Lock()
	p.closes++
	p.on = false
	p.mu.Unlock()
}

// end plays the sound out. The real player runs its listeners on a
// goroutine of its own; here they run on the test's, so what follows sees
// the settled state.
func (p *fakeSoundPlayer) end() {
	p.mu.Lock()
	p.on = false
	listeners := append([]func(){}, p.onEnd...)
	p.mu.Unlock()
	for _, fn := range listeners {
		fn()
	}
}

func (p *fakeSoundPlayer) counts() (played, pauses, stops, closes int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.played, p.pauses, p.stops, p.closes
}

// fakePlayerFactory records every player the app builds.
type fakePlayerFactory struct {
	mu   sync.Mutex
	made []*fakeSoundPlayer
}

func (f *fakePlayerFactory) new(snd *audio.Sound, volume float64) soundPlayer {
	p := &fakeSoundPlayer{volume: volume}
	f.mu.Lock()
	f.made = append(f.made, p)
	f.mu.Unlock()
	return p
}

func (f *fakePlayerFactory) players() []*fakeSoundPlayer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeSoundPlayer(nil), f.made...)
}

func (f *fakePlayerFactory) last(t *testing.T) *fakeSoundPlayer {
	t.Helper()
	ps := f.players()
	if len(ps) == 0 {
		t.Fatal("no player was built")
	}
	return ps[len(ps)-1]
}

// soundPlayers makes the app build fakes instead of playing through the
// audio output, for the length of the test.
func soundPlayers(t *testing.T) *fakePlayerFactory {
	t.Helper()
	f := &fakePlayerFactory{}
	old := newSoundPlayer
	newSoundPlayer = f.new
	t.Cleanup(func() { newSoundPlayer = old })
	return f
}

// seedSound puts a decoded sound in the cache under path, so auditions
// and reminders reach for it without reading or decoding a file.
func seedSound(t *testing.T, path string) {
	t.Helper()
	soundCache.Store(path, audio.NewSound(8000, 1, []float32{0, 0, 0, 0}))
	t.Cleanup(func() { soundCache.Delete(path) })
}

// newSoundApp builds an app whose settings name the custom sounds, at a
// volume the tests can recognize.
func newSoundApp(t *testing.T, breakPath, hydrationPath string) *App {
	t.Helper()
	a, err := NewForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.set.Update(func(s *store.Settings) {
		s.EnableSound = true
		s.SoundVolume = 0.5
		s.CustomBreakSoundPath = breakPath
		s.CustomHydrationSoundPath = hydrationPath
	}); err != nil {
		t.Fatal(err)
	}
	return a
}

// ---- auditions ----

// Clicking 试听 again while the sound plays pauses it, and the next click
// resumes it: a second player must never start over the first.
func TestSoundAuditionPausesAndResumes(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "")

	a.TestSound("break")
	players := f.players()
	if len(players) != 1 {
		t.Fatalf("players after the first audition = %d, want 1", len(players))
	}
	p := players[0]
	if p.volume != 0.5 {
		t.Errorf("audition volume = %v, want 0.5", p.volume)
	}
	if playing, paused := a.SoundPreviewState("break"); !playing || paused {
		t.Errorf("state after play = playing %v paused %v, want playing", playing, paused)
	}

	a.TestSound("break") // pause
	if n := len(f.players()); n != 1 {
		t.Errorf("players after the pause click = %d, want 1 (paused, not stacked)", n)
	}
	if playing, paused := a.SoundPreviewState("break"); playing || !paused {
		t.Errorf("state while paused = playing %v paused %v, want paused", playing, paused)
	}

	a.TestSound("break") // resume
	if n := len(f.players()); n != 1 {
		t.Errorf("players after the resume click = %d, want 1", n)
	}
	if played, pauses, _, _ := p.counts(); played != 2 || pauses != 1 {
		t.Errorf("player plays = %d pauses = %d, want 2 and 1 (resumed, not restarted)", played, pauses)
	}
	if playing, paused := a.SoundPreviewState("break"); !playing || paused {
		t.Errorf("state after resume = playing %v paused %v, want playing", playing, paused)
	}
}

// One audition plays at a time: the other kind's row stops this one
// before its own starts.
func TestSoundAuditionSwitchesKind(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	seedSound(t, "/tmp/restify-hydration.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "/tmp/restify-hydration.mp3")

	a.TestSound("break")
	first := f.last(t)
	a.TestSound("hydration")

	if len(f.players()) != 2 {
		t.Fatalf("players after switching = %d, want 2", len(f.players()))
	}
	if _, _, stops, closes := first.counts(); stops == 0 || closes == 0 {
		t.Errorf("break audition stops = %d closes = %d, want both after switching", stops, closes)
	}
	if playing, paused := a.SoundPreviewState("break"); playing || paused {
		t.Errorf("break state after switching = playing %v paused %v, want idle", playing, paused)
	}
	if playing, paused := a.SoundPreviewState("hydration"); !playing || paused {
		t.Errorf("hydration state after switching = playing %v paused %v, want playing", playing, paused)
	}
}

// A callback from a player replaced since must not clear the audition
// now playing.
func TestSoundAuditionStaleEndIgnored(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	seedSound(t, "/tmp/restify-hydration.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "/tmp/restify-hydration.mp3")

	a.TestSound("break")
	old := f.last(t)
	a.TestSound("hydration")
	old.end()

	if playing, paused := a.SoundPreviewState("hydration"); !playing || paused {
		t.Errorf("stale end reset the new audition: playing %v paused %v", playing, paused)
	}
}

// When the sound plays out, the button says 试听 again, and the next click
// starts a new player from the beginning.
func TestSoundAuditionEnds(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "")

	a.TestSound("break")
	f.last(t).end()
	if playing, paused := a.SoundPreviewState("break"); playing || paused {
		t.Errorf("state after the end = playing %v paused %v, want idle", playing, paused)
	}

	a.TestSound("break")
	if len(f.players()) != 2 {
		t.Fatalf("players after replaying = %d, want 2", len(f.players()))
	}
	if !f.last(t).Playing() {
		t.Error("the replay is not playing")
	}
}

// The audition leaves the reminder sounds alone.
func TestSoundAuditionKeepsEventSounds(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	seedSound(t, "/tmp/restify-hydration.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "/tmp/restify-hydration.mp3")

	a.playSound("hydration", 0.8)
	event := f.last(t)
	for range 3 { // play, pause, resume
		a.TestSound("break")
	}
	if _, _, stops, _ := event.counts(); stops != 0 {
		t.Errorf("the hydration reminder was stopped %d times by an audition, want 0", stops)
	}
	if !event.Playing() {
		t.Error("the hydration reminder stopped when the break sound was auditioned")
	}
}

// ---- state ----

// No audition, or one of another kind, reads as idle.
func TestSoundPreviewStateIdle(t *testing.T) {
	a := newSoundApp(t, "", "")
	for _, kind := range []string{"break", "hydration", "unknown"} {
		if playing, paused := a.SoundPreviewState(kind); playing || paused {
			t.Errorf("%s state = playing %v paused %v before any audition", kind, playing, paused)
		}
	}
}

// ---- reminder sounds ----

// A new reminder sound of a kind replaces the one still playing, at that
// event's share of the volume; another kind's sound keeps playing.
func TestPlaySoundReplacesSameKind(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	seedSound(t, "/tmp/restify-hydration.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "/tmp/restify-hydration.mp3")

	a.playSound("break", 1.0)
	firstBreak := f.last(t)
	a.playSound("hydration", 0.8)
	hydration := f.last(t)
	a.playSound("break", 1.0)

	if n := len(f.players()); n != 3 {
		t.Fatalf("players = %d, want 3 (one per play, the earlier break replaced)", n)
	}
	if _, _, stops, closes := firstBreak.counts(); stops == 0 || closes == 0 {
		t.Errorf("the first break sound stops = %d closes = %d, want both", stops, closes)
	}
	if !hydration.Playing() {
		t.Error("the hydration reminder stopped when the break reminder replaced its own")
	}
	if got, want := hydration.volume, 0.5*0.8; got != want {
		t.Errorf("hydration volume = %v, want %v", got, want)
	}
	if got, want := firstBreak.volume, 0.5; got != want {
		t.Errorf("break volume = %v, want %v", got, want)
	}
}

// ---- endRest ----

// Leaving the rest stops the break reminder and the audition; the drink
// chime, which is not the rest's, keeps playing.
func TestEndRestStopsBreakSound(t *testing.T) {
	f := soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	seedSound(t, "/tmp/restify-hydration.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "/tmp/restify-hydration.mp3")

	a.playSound("break", 1.0)
	breakSound := f.last(t)
	a.playSound("hydration", 0.8)
	hydration := f.last(t)
	a.TestSound("break")
	audition := f.last(t)

	a.EndRest()

	if _, _, stops, closes := breakSound.counts(); stops == 0 || closes == 0 {
		t.Errorf("break sound stops = %d closes = %d after the rest, want both", stops, closes)
	}
	if _, _, stops, closes := audition.counts(); stops == 0 || closes == 0 {
		t.Errorf("audition stops = %d closes = %d after the rest, want both", stops, closes)
	}
	if _, _, stops, _ := hydration.counts(); stops != 0 {
		t.Errorf("hydration sound stopped %d times with the rest, want 0", stops)
	}
	if !hydration.Playing() {
		t.Error("the drink chime stopped when the rest ended")
	}
	if playing, paused := a.SoundPreviewState("break"); playing || paused {
		t.Errorf("audition after the rest = playing %v paused %v, want idle", playing, paused)
	}
}

// A rest ending twice (a force exit racing the countdown) is harmless.
func TestEndRestTwice(t *testing.T) {
	soundPlayers(t)
	seedSound(t, "/tmp/restify-break.mp3")
	a := newSoundApp(t, "/tmp/restify-break.mp3", "")
	a.playSound("break", 1.0)
	a.EndRest()
	a.EndRest()
}
