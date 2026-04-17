package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Colours ─────────────────────────────────────────────────────────────────

const (
	colorBrand   = lipgloss.Color("86")  // GopherCI cyan
	colorWhite   = lipgloss.Color("15")  // bright white
	colorDim     = lipgloss.Color("241") // muted gray – unselected text
	colorFaint   = lipgloss.Color("237") // very dark – separators
	colorSuccess = lipgloss.Color("42")  // green
	colorFailed  = lipgloss.Color("196") // red
	colorRunning = lipgloss.Color("214") // amber
	colorPending = lipgloss.Color("33")  // blue
)

// ── Base styles ──────────────────────────────────────────────────────────────

var (
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// Selected list-item label
	selectedTextStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite)

	// Cursor glyph rendered in brand cyan
	cursorGlyph = lipgloss.NewStyle().Foreground(colorBrand).Render("❯")

	// Status dots
	dotSuccess = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	dotFailed  = lipgloss.NewStyle().Foreground(colorFailed).Render("●")
	dotRunning = lipgloss.NewStyle().Foreground(colorRunning).Render("●")
	dotPending = lipgloss.NewStyle().Foreground(colorPending).Render("●")

	// Feedback
	errorStyle   = lipgloss.NewStyle().Foreground(colorFailed).Bold(true)
	loadingStyle = lipgloss.NewStyle().Foreground(colorBrand).Italic(true)
	helpStyle    = lipgloss.NewStyle().Foreground(colorDim)
	sepStyle     = lipgloss.NewStyle().Foreground(colorFaint)
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// renderItem renders a list row with a cursor prefix.
//
//	selected → "❯ Label"  (cyan cursor, bold white label)
//	normal   → "  Label"  (no cursor, dim label)
func renderItem(label string, selected bool) string {
	if selected {
		return cursorGlyph + " " + selectedTextStyle.Render(label)
	}
	return dimStyle.Render("  " + label)
}

// statusDot returns a coloured ● dot matching the job status.
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

// renderSep renders a subtle horizontal divider line.
func renderSep() string {
	return sepStyle.Render("  " + strings.Repeat("─", 44))
}

// renderHelp renders keyboard hint pairs at the bottom of a view.
// Pass key-action pairs e.g. "↑↓ move", "enter select".
func renderHelp(hints ...string) string {
	return helpStyle.Render("  " + strings.Join(hints, "  ·  "))
}
