# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Restify is a native macOS menu-bar app for healthy work and hydration habits: a work timer that forces a full-screen rest when it ends, and periodic drink reminders with daily goals and statistics. It is written in Go on the [mygo](https://github.com/egoist/mygo) framework with MyGo's native UI, and the interface is built on [Lobe MyGo](https://github.com/lobehub/lobe-mygo) in the Lobe UI design language. `go.mod` pins `github.com/egoist/mygo` v0.8.4-0.20261108082913-9233663736ca — the version Lobe MyGo requires — and `github.com/lobehub/lobe-mygo` v0.0.0-20261109063003-6081a16a09f2. All user-facing copy is Simplified Chinese.

## Layout

- `main.go` — entry: single-instance lock, notification routing, lifecycle
- `internal/store/` — JSON persistence in `~/Library/Application Support/Restify/` (settings, hydration records with 30-day retention, timer state)
- `internal/core/` — pure-Go engines, fully unit-tested, no UI deps: work timer (idle auto-pause, smart nudge, restore-across-launch), hydration loop (gates, midnight reset, 30s throttle), rest session, message picking
- `internal/gui/` — the Lobe MyGo interface: `theme.go` applies Lobe's tokens through `theme.Apply` (`applyTheme`/`applyDarkTheme` return `*theme.Tokens`); `components.go` holds the shared component vocabulary (Panel, Card, SettingsCard, SettingRow, BigValue, Metrics, ProgressBar, Tag, Notice, EmptyState, PageTitle, Brand, Accent, `percentLabel`); `icons.go` maps the in-window icons to Lobe MyGo's Lucide set; `model.go` defines the small model interfaces the views read. Views: `view_popover.go` (tray panel), `view_settings.go`, `view_stats.go`, `view_alerts.go` (drink-reminder popup and rest overlay), with headless view tests and a component gallery (`gallery_test.go`; `RESTIFY_GALLERY_DIR=/tmp/gallery go test ./internal/gui/` writes its PNGs).
- `internal/app/` — wiring: tray + flyout, windows, notifications, sounds, global shortcuts, login item; `update.go` mirrors mygo's updater plugin state under `App.mu` for the views
- `tools/preview` — renders the views headlessly to PNG for visual checks: `go run ./tools/preview -out /tmp/preview` (also `-view`, `-dark`, `-measure`)
- `tools/gen_icons.sh` and `tools/gen_tray` regenerate `internal/gui/glyphs_gen.go` (menu-bar glyphs from Remix Icon) and the tray template PNGs — rerun after icon changes; `tools/gen_icon` renders the app icon (`go run ./tools/gen_icon -o resources/icon.png quiet-ring`)

## Build and Run

```bash
go tool mygo dev       # develop with live reload
go test ./... -race    # engine unit tests + headless view tests
go tool mygo vet .     # mygo-specific checks (build lifetimes)
go tool mygo build     # .app + .dmg into build/
go run ./tools/preview -out /tmp/preview   # render the views headlessly to PNG
```

Requires Go ≥ 1.27.3 (toolchain auto-switches). The mygo CLI is
declared as a tool dependency in `go.mod` (`tool github.com/egoist/mygo/cmd/mygo`),
so `go tool mygo` needs no separate install.

## Conventions

- Build the interface from Lobe MyGo components, and use only the components and props its reference documents — https://mygo.lobehub.com and the module's `catalog/`; do not invent props or tokens. The UI is Lobe MyGo throughout; there is no second design system or legacy token set to follow.
- Apply Lobe's tokens once per view through `applyTheme` (or `applyDarkTheme` for the rest overlay), and take colors from the `Accent` palette so the light and dark appearances follow; spacing and motion come from the shared constants and presets in `components.go`.
- Paint each view's own surface from the tokens (`BgLayout` for a page, `BgContainer` for the tray panel); do not leave a view transparent over the window's native material. The material's appearance and the tokens' can disagree — the panel then shows light text on a light backdrop — so the panel paints the container color instead of relying on the popover material.
- A `feedback.Progress` that shows its percent needs `Format: percentLabel`: Lobe's default formatter writes the raw float, which a 5/12 ring renders as `41.66666%` and clips.
- Keep `mygo` and `lobe-mygo` in step: Lobe MyGo requires the exact mygo commit pinned in `go.mod`, so upgrade them together and re-run `go tool mygo vet .` and the previews.
- Engine logic goes in `internal/core` with tests; UI wiring in `internal/app`; views in `internal/gui` behind small model interfaces so they stay headless-testable.
- Views read state through the model interfaces (`internal/gui/model.go`); state mutations call the engines, whose hooks call `tickUI()` to redraw windows and refresh the tray.
- macOS notifications only work from packaged builds (`mygo dev`/`mygo build`), and mygo notifications have no action buttons — clicking one opens the in-app popup instead.
- The rest overlay spans each display's whole bounds, the menu bar and the Dock included. AppKit clamps a titled window (`Frameless` included) to the work area, so `coverRestDisplay` (`internal/app/overlay_darwin.go`) makes each cover borderless and raises it over the menu bar through AppKit's own calls (purego); drop that escape once mygo passes a window level through.
- Sounds (`internal/app/notify.go`) play through one player per reminder kind plus one audition player: a new sound replaces its slot instead of stacking, the settings rows' 试听 button pauses and resumes the audition in place, and `endRest` stops the rest's audio.
- The app is an accessory (no Dock icon); the tray keeps it alive (`OnWindowAllClosed` is a no-op).
- Updates: the mygo updater plugin (`native.Plugin`, Simplified Chinese) is registered in `main.go` and feeds from `updates` in `mygo.json` (signed GitHub releases of `lichunqiang/restify`, tag prefix `v`). The plugin's functions panic before `mygo.Use`, so views never call them — `internal/app/update.go` mirrors the plugin state (auto-check, last check, available version) under `mu`, seeded in `Start` via `initUpdates` and kept fresh by `updater.OnChange`; its preferences live in the plugin's own `updater.json` in the user-data dir (not `internal/store`). Version bumps touch three places at once: `app.Version` (`internal/app/app.go`), `version` in `mygo.json`, and a `## <version>` section in `CHANGELOG.md` (the release notes; a `v*` tag pushes publish via `.github/workflows/release.yml` with the `MYGO_UPDATER_PRIVATE_KEY` secret).
- Icons: in-window icons come from Lobe MyGo's Lucide set (`internal/gui/icons.go`); the menu-bar images are generated Remix Icon template PNGs (black on transparent, `internal/gui/glyphs_gen.go` through `tools/gen_icons.sh` + `tools/gen_tray`).

## History

The app was originally SwiftUI (macOS 13+). The Swift implementation was removed; find it in git history under `Restify/`. The Go interface has since been rebuilt on Lobe MyGo's Lobe UI, and that migration is complete. Data does not migrate from the old UserDefaults storage.