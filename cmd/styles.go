package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Colours ──────────────────────────────────────────────────────────────────

const (
	colorBrand   = lipgloss.Color("86")  // GopherCI cyan
	colorWhite   = lipgloss.Color("15")  // bright white
	colorDim     = lipgloss.Color("241") // muted gray
	colorFaint   = lipgloss.Color("237") // very dark – separators
	colorSuccess = lipgloss.Color("42")  // green
	colorFailed  = lipgloss.Color("196") // red
	colorRunning = lipgloss.Color("214") // amber
	colorPending = lipgloss.Color("33")  // blue
	colorWarning = lipgloss.Color("220") // yellow
	colorMascot  = lipgloss.Color("75")  // light blue – mascot
)

// ── Base styles ──────────────────────────────────────────────────────────────

// Scaled from 20-wide/5-row original → 14 chars wide, 5 rows tall.
// Structure: antennas / eye-tops / arms+eye-bottoms / body / legs.
const mascotASCII = "" +
	" ██        ██ \n" +
	" ██▀▀████▀▀██ \n" +
	"▄██▄▄████▄▄██▄\n" +
	" ████████████ \n" +
	" ██        ██ "

var (
	mascotStyle = lipgloss.NewStyle().Foreground(colorMascot)

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	selectedTextStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite)

	cursorGlyph = lipgloss.NewStyle().Foreground(colorBrand).Render("❯")

	dotSuccess = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	dotFailed  = lipgloss.NewStyle().Foreground(colorFailed).Render("●")
	dotRunning = lipgloss.NewStyle().Foreground(colorRunning).Render("●")
	dotPending = lipgloss.NewStyle().Foreground(colorPending).Render("●")

	errorStyle   = lipgloss.NewStyle().Foreground(colorFailed).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	warnStyle    = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	loadingStyle = lipgloss.NewStyle().Foreground(colorBrand).Italic(true)
	helpStyle    = lipgloss.NewStyle().Foreground(colorDim)
	sepStyle     = lipgloss.NewStyle().Foreground(colorFaint)
	monoStyle    = lipgloss.NewStyle().Foreground(colorDim).Italic(true)

	confirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorWarning).
			Padding(0, 2).
			MarginLeft(2)

	breadcrumbSepStyle  = lipgloss.NewStyle().Foreground(colorDim)
	breadcrumbCurrStyle = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	breadcrumbPrevStyle = lipgloss.NewStyle().Foreground(colorDim)
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// renderItem renders a list row: selected = "❯ Label", normal = "  Label".
func renderItem(label string, selected bool) string {
	if selected {
		return cursorGlyph + " " + selectedTextStyle.Render(label)
	}
	return dimStyle.Render("  " + label)
}

// statusDot returns a coloured ● matching the job status.
func statusDot(status string) string {
	switch status {
	case "success":
		return dotSuccess
	case "failed":
		return dotFailed
	case "running":
		return dotRunning
	default:
		return dotPending
	}
}

// statusText returns the status word styled in its matching colour.
func statusText(status string) string {
	switch status {
	case "success":
		return lipgloss.NewStyle().Foreground(colorSuccess).Render(status)
	case "failed":
		return lipgloss.NewStyle().Foreground(colorFailed).Render(status)
	case "running":
		return lipgloss.NewStyle().Foreground(colorRunning).Render(status)
	default:
		return dimStyle.Render(status)
	}
}

// renderSep renders a subtle horizontal divider line (fixed 44 chars).
func renderSep() string {
	return sepStyle.Render("  " + strings.Repeat("─", 44))
}

// renderSepWidth renders a divider scaled to the terminal width.
func renderSepWidth(w int) string {
	n := w - 4
	if n < 4 {
		n = 44
	}
	return sepStyle.Render("  " + strings.Repeat("─", n))
}

// renderHelp renders keyboard hint pairs at the bottom of a view.
func renderHelp(hints ...string) string {
	return helpStyle.Render("  " + strings.Join(hints, "  ·  "))
}
