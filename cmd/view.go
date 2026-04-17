package main

import (
	"fmt"
	"strings"
)

func (m model) View() string {
	switch m.screen {
	case screenLogin:
		return viewLogin(m)
	case screenMenu:
		return viewMenu(m)
	case screenJobs:
		return viewJobs(m)
	case screenJobsActions:
		return viewJobsActions(m)
	case screenLogs:
		return viewLogs(m)
	case screenSSHKeys:
		return viewSSHKeys(m)
	}
	return ""
}

// ── Login ────────────────────────────────────────────────────────────────────

func viewLogin(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + brandStyle.Render("GopherCI") + "\n\n")

	options := []string{"Login with email", "Login with GitHub"}
	for i, opt := range options {
		b.WriteString("  " + renderItem(opt, i == m.loginCursor) + "\n")
	}

	if m.loginCursor == 0 {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render("Email") + "\n")
		b.WriteString("  " + m.inputs[0].View() + "\n\n")
		b.WriteString("  " + dimStyle.Render("Password") + "\n")
		b.WriteString("  " + m.inputs[1].View() + "\n")
	}

	if m.err != "" {
		b.WriteString("\n  " + errorStyle.Render("✖  "+m.err) + "\n")
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ navigate", "enter select", "tab switch field") + "\n")
	return b.String()
}

// ── Menu ─────────────────────────────────────────────────────────────────────

func viewMenu(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + brandStyle.Render("GopherCI") + "\n\n")

	items := []string{"Jobs", "SSH Keys", "Logout"}
	for i, item := range items {
		b.WriteString("  " + renderItem(item, i == m.menuCursor) + "\n")
	}

	if m.err != "" {
		b.WriteString("\n  " + errorStyle.Render("✖  "+m.err) + "\n")
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ navigate", "enter select", "ctrl+c quit") + "\n")
	return b.String()
}

// ── Jobs ─────────────────────────────────────────────────────────────────────

func viewJobs(m model) string {
	if m.loading {
		return "\n  " + loadingStyle.Render("⠸  "+m.loadingMsg) + "\n"
	}
	if m.showOrgPicker {
		return viewOrgPicker(m)
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render("Jobs") + "\n\n")

	if len(m.jobs) == 0 {
		b.WriteString("  " + dimStyle.Render("No jobs found") + "\n")
	} else {
		for i, job := range m.jobs {
			cursor := " "
			if i == m.jobCursor {
				cursor = cursorGlyph
			}
			id := dimStyle.Render(job.ID[:8])
			st := dimStyle.Render(job.Status)
			if i == m.jobCursor {
				id = selectedTextStyle.Render(job.ID[:8])
				st = statusText(job.Status)
			}
			b.WriteString(fmt.Sprintf("  %s %s  %s  %s\n",
				cursor, statusDot(job.Status), id, st))
		}
	}

	if m.err != "" {
		b.WriteString("\n  " + errorStyle.Render("✖  "+m.err) + "\n")
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ move", "enter open", "n new", "esc back") + "\n")
	return b.String()
}

// ── Org / repo picker ────────────────────────────────────────────────────────

func viewOrgPicker(m model) string {
	var b strings.Builder
	b.WriteString("\n")

	switch {
	case len(m.githubRepos) > 0:
		b.WriteString("  " + headerStyle.Render("Select Repository") + "\n\n")
		currentOwner := ""
		for i, repo := range m.githubRepos {
			parts := strings.SplitN(repo.FullName, "/", 2)
			owner := parts[0]
			if owner != currentOwner {
				currentOwner = owner
				b.WriteString("\n  " + dimStyle.Render(owner+"/") + "\n")
			}
			b.WriteString("  " + renderItem(repo.Name, i == m.githubRepoCursor) + "\n")
		}

	case len(m.organizations) == 0:
		b.WriteString("  " + dimStyle.Render("No organizations found") + "\n")

	default:
		b.WriteString("  " + headerStyle.Render("Select Organization") + "\n\n")
		for i, org := range m.organizations {
			b.WriteString("  " + renderItem(org.Name, i == m.orgCursor) + "\n")
		}
		if len(m.repositories) > 0 {
			b.WriteString("\n  " + headerStyle.Render("Select Repository") + "\n\n")
			for i, repo := range m.repositories {
				b.WriteString("  " + renderItem(repo.Name, i == m.repoCursor) + "\n")
			}
		}
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ navigate", "enter select", "esc cancel") + "\n")
	return b.String()
}

// ── Job actions ──────────────────────────────────────────────────────────────

func viewJobsActions(m model) string {
	if m.selectedJob == nil {
		return "\n  " + dimStyle.Render("No job selected") + "\n\n" +
			renderSep() + "\n" + renderHelp("esc back") + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render("Job "+m.selectedJob.ID[:8]) + "\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		statusDot(m.selectedJob.Status),
		statusText(m.selectedJob.Status)))

	actions := []string{"View Logs", "Re-run", "Delete"}
	for i, action := range actions {
		b.WriteString("  " + renderItem(action, i == m.actionCursor) + "\n")
	}

	if m.err != "" {
		b.WriteString("\n  " + errorStyle.Render("✖  "+m.err) + "\n")
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ navigate", "enter confirm", "esc back") + "\n")
	return b.String()
}

// ── Logs ─────────────────────────────────────────────────────────────────────

func viewLogs(m model) string {
	if m.selectedJob == nil {
		return "\n  " + dimStyle.Render("No job selected") + "\n\n" +
			renderSep() + "\n" + renderHelp("esc back") + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n\n",
		headerStyle.Render("Logs"),
		dimStyle.Render(m.selectedJob.ID[:8]),
		statusDot(m.selectedJob.Status),
		statusText(m.selectedJob.Status)))

	if len(m.selectedJob.Logs) == 0 {
		b.WriteString("  " + dimStyle.Render("No logs available") + "\n")
	} else {
		for _, log := range m.selectedJob.Logs {
			b.WriteString(log + "\n")
		}
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("esc back") + "\n")
	return b.String()
}

// ── SSH Keys ─────────────────────────────────────────────────────────────────

func viewSSHKeys(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render("SSH Keys") + "\n\n")
	b.WriteString("  " + dimStyle.Render("coming soon") + "\n")
	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("esc back") + "\n")
	return b.String()
}
