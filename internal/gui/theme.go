// Package gui holds Restify's interface in the Lobe UI design language,
// built with the Lobe MyGo components: neutral surfaces, one palette color
// naming the parts of the app, soft filled cards, large figures over small
// labels and capsule controls.
//
// Every view applies the tokens through applyTheme and draws with the Lobe
// MyGo components, so colors, radii, type and motion come from one system.
package gui

import (
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

// applyTheme applies Lobe UI's tokens in the appearance the system — or the
// settings, through mygo.Theme — names, and reports them.
//
// Views paint their own surfaces from the tokens. Lobe UI's surfaces are
// opaque, and the tray panel's window material takes its appearance from
// the same setting as the tokens: leaving the panel transparent would let
// the two disagree (light text over a light material) whenever the
// material does not follow, so the panel paints the container color.
func applyTheme(c *ui.Context) *theme.Tokens {
	theme.Apply(c)
	return theme.TokensOf(c)
}

// applyDarkTheme uses the dark appearance whatever the system says, for the
// rest overlay: it covers the screen the way a rest screen does.
func applyDarkTheme(c *ui.Context) *theme.Tokens {
	dark := theme.Dark()
	c.SetTheme(dark.Theme())
	return dark
}
