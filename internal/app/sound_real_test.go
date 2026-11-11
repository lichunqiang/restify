package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"restify/internal/store"
)

// writeSilentWAV writes a silent 16-bit PCM WAV of the given length: a
// sound the real audio package decodes and plays with no noise.
func writeSilentWAV(t *testing.T, path string, seconds float64) {
	t.Helper()
	const rate, channels, bits = 44100, 1, 16
	data := make([]byte, int(seconds*rate)*channels*bits/8)
	b := make([]byte, 0, 44+len(data))
	put16 := func(v int) { b = append(b, byte(v), byte(v>>8)) }
	put32 := func(v int) { b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	b = append(b, "RIFF"...)
	put32(36 + len(data))
	b = append(b, "WAVEfmt "...)
	put32(16)
	put16(1) // PCM
	put16(channels)
	put32(rate)
	put32(rate * channels * bits / 8)
	put16(channels * bits / 8)
	put16(bits)
	b = append(b, "data"...)
	put32(len(data))
	b = append(b, data...)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// soundApp names a real WAV as the break and drink sound, at a volume too
// low to notice, on an app whose audio plays through the audio package
// itself (players keep time unheard where no device opens, as in CI).
func soundApp(t *testing.T, breakPath, hydrationPath string) *App {
	t.Helper()
	a, err := NewForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The store directly: SetSetting also applies the theme through mygo,
	// which waits for an app loop that tests do not run.
	if err := a.set.Update(func(s *store.Settings) {
		s.EnableSound = true
		s.SoundVolume = 0.05
		s.CustomBreakSoundPath = breakPath
		s.CustomHydrationSoundPath = hydrationPath
	}); err != nil {
		t.Fatal(err)
	}
	return a
}

// waitState polls the audition's state until want reports it settled, or
// the timeout passes.
func waitState(t *testing.T, a *App, want func(playing, paused bool) bool) (playing, paused bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		playing, paused = a.SoundPreviewState("break")
		if want(playing, paused) {
			return playing, paused
		}
		if time.Now().After(deadline) {
			t.Fatalf("audition state = playing %v paused %v after %v", playing, paused, 3*time.Second)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A custom sound plays through mygo's real player: a second click pauses
// it where it is, a third resumes it there, and its end puts the row back
// to 试听 — the fake-player tests check the slots, this one the player.
func TestSoundAuditionRealPlayer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "break.wav")
	writeSilentWAV(t, path, 0.6)
	a := soundApp(t, path, "")

	a.TestSound("break")
	if playing, paused := a.SoundPreviewState("break"); !playing || paused {
		t.Fatalf("state after the first click = playing %v paused %v, want playing", playing, paused)
	}
	a.TestSound("break") // pause
	if playing, paused := a.SoundPreviewState("break"); playing || !paused {
		t.Fatalf("state after the second click = playing %v paused %v, want paused", playing, paused)
	}
	a.TestSound("break") // resume where it paused
	if playing, paused := a.SoundPreviewState("break"); !playing || paused {
		t.Fatalf("state after the third click = playing %v paused %v, want playing", playing, paused)
	}
	// The sound ends on its own, and the row goes back to 试听.
	waitState(t, a, func(playing, paused bool) bool { return !playing && !paused })
}

// Leaving the rest stops the break reminder's real player; the drink
// chime, which is not the rest's, plays on.
func TestEndRestStopsRealBreakSound(t *testing.T) {
	dir := t.TempDir()
	breakPath := filepath.Join(dir, "break.wav")
	hydrationPath := filepath.Join(dir, "hydration.wav")
	writeSilentWAV(t, breakPath, 3)
	writeSilentWAV(t, hydrationPath, 3)
	a := soundApp(t, breakPath, hydrationPath)

	a.playSound("break", 1.0)
	a.playSound("hydration", 0.8)
	a.soundMu.Lock()
	breakVoice, hydrationVoice := a.sounds["break"], a.sounds["hydration"]
	a.soundMu.Unlock()
	if breakVoice == nil || !breakVoice.player.Playing() {
		t.Fatal("the break sound did not start")
	}
	if hydrationVoice == nil || !hydrationVoice.player.Playing() {
		t.Fatal("the drink chime did not start")
	}

	a.EndRest()

	if breakVoice.player.Playing() {
		t.Error("the break sound still plays after the rest")
	}
	if a.sounds["break"] != nil {
		t.Error("the break sound is still slotted after the rest")
	}
	if !hydrationVoice.player.Playing() {
		t.Error("the drink chime stopped when the rest ended")
	}
}
