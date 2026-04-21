package main

import (
	"fmt"
	"strings"

	"github.com/lucaspose/goci-cli/internal/api"
)

// renderBreadcrumb builds a breadcrumb like "Menu  ❯  SSH Keys".
// The last segment is rendered bold white; previous ones are dim.
func renderBreadcrumb(segments ...string) string {
	if len(segments) == 0 {
		return ""
	}
	sep := breadcrumbSepStyle.Render("  ❯  ")
	parts := make([]string, len(segments))
	for i, s := range segments {
		if i == len(segments)-1 {
			parts[i] = breadcrumbCurrStyle.Render(s)
		} else {
			parts[i] = breadcrumbPrevStyle.Render(s)
		}
	}
	return "  " + strings.Join(parts, sep)
}

// renderConfirm renders a confirmation dialog box with a question and help hints.
func renderConfirm(question string, hints ...string) string {
	inner := warnStyle.Render("⚠  "+question) + "\n\n" +
		renderHelp(hints...)
	return confirmBoxStyle.Render(inner)
}

// renderPagination renders "Page 2/5  ·  pgdn/pgup naviguer".
func renderPagination(current, total int) string {
	return renderPaginationWithHint(current, total, "pgdn/pgup changer de page")
}

// renderPaginationWithHint renders "Page 2/5" with an optional hint.
func renderPaginationWithHint(current, total int, hint string) string {
	if total <= 1 {
		return ""
	}
	text := fmt.Sprintf("  Page %d/%d", current, total)
	hint = strings.TrimSpace(hint)
	if hint != "" {
		text += "  ·  " + hint
	}
	return dimStyle.Render(text)
}

// renderEmpty renders a friendly empty-state with a message and usage hint.
func renderEmpty(message, hint string) string {
	var b strings.Builder
	b.WriteString("  " + dimStyle.Render(message) + "\n")
	if hint != "" {
		b.WriteString("  " + dimStyle.Render(hint) + "\n")
	}
	return b.String()
}

// paginateJobs returns a page of jobs starting at offset, and the total page count.
func paginateJobs(jobs []api.Job, pageSize, offset int) ([]api.Job, int) {
	if pageSize <= 0 || len(jobs) == 0 {
		return jobs, 1
	}
	totalPages := (len(jobs) + pageSize - 1) / pageSize
	end := offset + pageSize
	if end > len(jobs) {
		end = len(jobs)
	}
	if offset >= len(jobs) {
		return nil, totalPages
	}
	return jobs[offset:end], totalPages
}

// paginateRange returns [start:end] for a list length and normalizes the page offset.
func paginateRange(totalItems, pageSize, offset int) (start, end, totalPages, normalizedOffset int) {
	if totalItems <= 0 {
		return 0, 0, 1, 0
	}

	if pageSize <= 0 {
		return 0, totalItems, 1, 0
	}

	totalPages = (totalItems + pageSize - 1) / pageSize
	maxOffset := (totalPages - 1) * pageSize
	normalizedOffset = clamp(offset, 0, maxOffset)

	start = normalizedOffset
	end = start + pageSize
	if end > totalItems {
		end = totalItems
	}

	return start, end, totalPages, normalizedOffset
}

// filterJobs returns only jobs matching status. Empty string means all jobs.
func filterJobs(jobs []api.Job, status string) []api.Job {
	if status == "" {
		return jobs
	}
	var out []api.Job
	for _, j := range jobs {
		if j.Status == status {
			out = append(out, j)
		}
	}
	return out
}

// formatDuration renders a duration in seconds as "1m 23s", "45s", or "—".
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "—"
	}
	m := seconds / 60
	s := seconds % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// shortID returns the first 8 characters of a job ID safely.
func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// truncate shortens s to at most n characters, appending "…" if trimmed.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
