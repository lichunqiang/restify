// Package store keeps Restify's data: settings, hydration records and the
// work timer's persisted state, each in one JSON file under the app's user
// data directory.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ThemeMode is the appearance the app follows.
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

func (m ThemeMode) DisplayName() string {
	switch m {
	case ThemeLight:
		return "浅色模式"
	case ThemeDark:
		return "深色模式"
	default:
		return "跟随系统"
	}
}

// IconStyle names the glyph the menu-bar item shows.
type IconStyle string

const (
	IconTimer    IconStyle = "timer"
	IconMoon     IconStyle = "moon"
	IconClock    IconStyle = "clock"
	IconHourglass IconStyle = "hourglass"
	IconDrop     IconStyle = "drop"
)

func (s IconStyle) DisplayName() string {
	switch s {
	case IconMoon:
		return "月亮"
	case IconClock:
		return "时钟"
	case IconHourglass:
		return "沙漏"
	case IconDrop:
		return "水滴"
	default:
		return "计时器"
	}
}

// Settings holds every user preference, as Restify's Swift version did.
type Settings struct {
	// Work
	WorkDuration   time.Duration `json:"workDuration"`   // per work period
	GracePeriod    time.Duration `json:"gracePeriod"`    // forced rest length
	EnableAutoLock bool          `json:"enableAutoLock"` // rest overlay on break

	// Hydration
	DailyGoal        int           `json:"dailyHydrationGoal"`
	ReminderInterval time.Duration `json:"hydrationReminderInterval"`
	EnableReminder   bool          `json:"enableHydrationReminder"`

	// Notification
	EnableSound    bool    `json:"enableSound"`
	SoundVolume    float64 `json:"soundVolume"` // 0..1
	EnablePopupAlert bool `json:"enablePopupAlert"` // show the in-app popup

	// General
	LaunchAtLogin   bool `json:"launchAtLogin"`
	ShowMenuBarIcon bool `json:"showMenuBarIcon"`
	AutoStartTimer  bool `json:"autoStartTimer"`

	// Appearance
	ThemeMode ThemeMode `json:"themeMode"`
	IconStyle IconStyle `json:"menuBarIconStyle"`

	// Custom sounds (absolute paths; empty plays the system beep)
	CustomBreakSoundPath     string `json:"customBreakSoundPath"`
	CustomHydrationSoundPath string `json:"customHydrationSoundPath"`

	// Reminder message libraries
	BreakMessageLibrary       []string `json:"breakMessageLibrary"`
	HydrationMessageLibrary   []string `json:"hydrationMessageLibrary"`
	EnableMotivationalMessages bool    `json:"enableMotivationalMessages"`

	// Smart work reminder
	EnableSmartReminder    bool          `json:"enableSmartWorkReminder"`
	SmartReminderThreshold time.Duration `json:"smartWorkReminderThreshold"`
}

// DefaultSettings mirrors the Swift app's AppSettings.default.
func DefaultSettings() Settings {
	return Settings{
		WorkDuration:            25 * time.Minute,
		GracePeriod:             5 * time.Minute,
		EnableAutoLock:          true,
		DailyGoal:               8,
		ReminderInterval:        time.Hour,
		EnableReminder:          true,
		EnableSound:             true,
		SoundVolume:             0.7,
		EnablePopupAlert:        true,
		LaunchAtLogin:           true,
		ShowMenuBarIcon:         true,
		AutoStartTimer:          false,
		ThemeMode:               ThemeSystem,
		IconStyle:               IconTimer,
		BreakMessageLibrary: []string{
			"休息时间到了！您已经工作了 {{duration}}，该休息一下了。",
			"该休息啦！站起来走走，放松一下眼睛。",
			"休息一下，为了更高效的工作！",
			"该喝杯水、活动活动了！",
			"暂停一下，深呼吸，放松身心。",
		},
		HydrationMessageLibrary: []string{
			"该喝水了！保持充足的水分对健康很重要。",
			"喝水时间到！补充水分，保持活力。",
			"记得喝水哦！身体需要水分。",
			"来一杯水吧，保持健康！",
		},
		EnableMotivationalMessages: true,
		EnableSmartReminder:        false,
		SmartReminderThreshold:     5 * time.Minute,
	}
}

// SettingsStore keeps Settings in settings.json, safe from any goroutine.
type SettingsStore struct {
	mu   sync.Mutex
	path string
	s    Settings
}

// OpenSettings loads settings.json in dir, creating it with the defaults.
func OpenSettings(dir string) (*SettingsStore, error) {
	st := &SettingsStore{
		path: filepath.Join(dir, "settings.json"),
		s:    DefaultSettings(),
	}
	data, err := os.ReadFile(st.path)
	switch {
	case os.IsNotExist(err):
		if err := st.saveLocked(); err != nil {
			return nil, err
		}
		return st, nil
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(data, &st.s); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	// Fields the file doesn't name keep their defaults.
	if st.s.BreakMessageLibrary == nil {
		st.s.BreakMessageLibrary = DefaultSettings().BreakMessageLibrary
	}
	if st.s.HydrationMessageLibrary == nil {
		st.s.HydrationMessageLibrary = DefaultSettings().HydrationMessageLibrary
	}
	return st, nil
}

// Get returns a copy of the settings.
func (st *SettingsStore) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update changes the settings under fn and saves them.
func (st *SettingsStore) Update(fn func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.s)
	return st.saveLocked()
}

func (st *SettingsStore) saveLocked() error {
	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(st.path, data)
}

// writeAtomic writes data to path without ever leaving a half-written file.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
