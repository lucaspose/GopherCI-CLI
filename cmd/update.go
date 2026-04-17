package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/config"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case loginSuccessMsg:
		m.config.Token = msg.token
		config.Save(m.config)
		m.apiClient.Token = msg.token
		m.screen = screenMenu
		return m, nil
	case jobsLoadedMsg:
		m.jobs = msg.jobs
		m.jobCursor = 0
		m.showOrgPicker = false
		m.loading = false
		return m, nil
	case orgsLoadedMsg:
		m.organizations = msg.orgs
		m.orgCursor = 0 // reset so stale cursor never goes out of bounds
		m.loading = false
		return m, nil
	case reposLoadedMsg:
		m.repositories = msg.repos
		m.repoCursor = 0 // reset on every new repo list
		m.loading = false
		return m, nil
	case jobDeletedMsg:
		m.screen = screenJobs
		m.loading = true
		m.loadingMsg = "Refreshing jobs..."
		return m, refreshJobs(m.apiClient, m)
	case jobCreatedMsg:
		m.screen = screenJobs
		m.loading = true
		m.loadingMsg = "Refreshing jobs..."
		return m, refreshJobs(m.apiClient, m)
	case githubTokenMsg:
		m.config.GitHubToken = msg.token
		config.Save(m.config)
		return m, exchangeGitHubToken(m.apiClient, msg.token)
	case githubReposLoadedMsg:
		m.githubRepos = msg.repos
		m.githubRepoCursor = 0 // reset on every new repo list
		m.loading = false
		return m, nil
	case errMsg:
		m.err = string(msg)
		m.loading = false
		if strings.Contains(string(msg), "401") {
			m.config.Token = ""
			config.Save(m.config)
			m.screen = screenLogin
		}
		return m, nil
	case tickMsg:
		// Re-arm the ticker so it keeps firing every 30s.
		cmd := startAuthTicker()
		if m.screen != screenLogin && !m.loading {
			return m, tea.Batch(cmd, checkAuthCmd(m.apiClient))
		}
		return m, cmd
	}
	// Forward other messages (e.g. text input ticks) to the focused input.
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.inputs[m.focused].Blur()
		m.focused = (m.focused + 1) % len(m.inputs)
		m.inputs[m.focused].Focus()
		return m, nil

	case "enter":
		return m.handleEnter()

	case "esc":
		return m.handleEsc()

	case "up":
		m.moveCursor(-1)

	case "down":
		m.moveCursor(+1)

	case "n":
		if m.screen == screenJobs {
			m.showOrgPicker = true
			m.loading = true
			m.loadingMsg = "Loading repositories..."
			return m, fetchOrgsOrGitHub(m.apiClient, m.config)
		}
	}
	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenLogin:
		if m.loginCursor == 1 {
			openGitHubLogin()
			return m, waitForGitHubToken()
		}
		return m, doLogin(m.apiClient, m.inputs[0].Value(), m.inputs[1].Value())

	case screenMenu:
		switch m.menuCursor {
		case 0:
			m.screen = screenJobs
			m.showOrgPicker = true
			m.loading = true
			m.loadingMsg = "Loading repositories..."
			return m, fetchOrgsOrGitHub(m.apiClient, m.config)
		case 1:
			m.screen = screenSSHKeys
		case 2:
			m.config.Token = ""
			config.Save(m.config)
			m.screen = screenLogin
		}
		return m, nil

	case screenJobs:
		if m.showOrgPicker {
			return m.handleOrgPickerEnter()
		}
		// Guard: only navigate when cursor is valid.
		if len(m.jobs) > 0 && m.jobCursor >= 0 && m.jobCursor < len(m.jobs) {
			m.selectedJob = &m.jobs[m.jobCursor]
			m.screen = screenJobsActions
			m.actionCursor = 0
		}
		return m, nil

	case screenJobsActions:
		return m.handleActionEnter()
	}
	return m, nil
}

func (m model) handleOrgPickerEnter() (tea.Model, tea.Cmd) {
	if len(m.githubRepos) > 0 {
		if m.githubRepoCursor < 0 || m.githubRepoCursor >= len(m.githubRepos) {
			return m, nil
		}
		selected := m.githubRepos[m.githubRepoCursor]
		m.selectedGitHubRepo = &selected
		m.showOrgPicker = false
		m.githubRepos = nil
		m.loading = true
		m.loadingMsg = "Loading jobs..."
		return m, fetchAllJobs(m.apiClient, selected.CloneURL)
	}
	// Org/repo two-step: first pick org, then pick repo.
	if len(m.repositories) == 0 {
		if m.orgCursor < 0 || m.orgCursor >= len(m.organizations) {
			return m, nil
		}
		m.selectedOrg = &m.organizations[m.orgCursor]
		m.loading = true
		m.loadingMsg = "Loading repositories..."
		return m, fetchRepos(m.apiClient, m.selectedOrg.ID)
	}
	if m.repoCursor < 0 || m.repoCursor >= len(m.repositories) {
		return m, nil
	}
	m.selectedRepo = &m.repositories[m.repoCursor]
	m.showOrgPicker = false
	m.loading = true
	m.loadingMsg = "Loading jobs..."
	return m, fetchJobs(m.apiClient, m.selectedOrg.ID, m.selectedRepo.ID)
}

func (m model) handleActionEnter() (tea.Model, tea.Cmd) {
	switch m.actionCursor {
	case 0:
		m.screen = screenLogs
	case 1:
		// Prefer the GitHub-repo context when available; fall back to org/repo.
		if m.selectedGitHubRepo != nil {
			return m, rerunJobWithURL(m.apiClient, m.selectedJob.CloneURL)
		}
		if m.selectedOrg != nil && m.selectedRepo != nil {
			return m, rerunJob(m.apiClient, m.selectedOrg.ID, m.selectedRepo.ID)
		}
		// No repo context at all – surface an error instead of silently failing.
		m.err = "no repository context for re-run"
		return m, nil
	case 2:
		return m, dispatchDelete(m.apiClient, m)
	}
	return m, nil
}

func (m model) handleEsc() (tea.Model, tea.Cmd) {
	m.err = ""
	switch {
	case m.showOrgPicker:
		m.showOrgPicker = false
		m.organizations = nil
		m.repositories = nil
		m.orgCursor = 0
		m.repoCursor = 0
		m.githubRepoCursor = 0
		// If no jobs were loaded yet go back to the menu, otherwise stay on
		// the jobs list so the user doesn't lose their context.
		if len(m.jobs) == 0 {
			m.screen = screenMenu
		}
	case m.screen == screenLogs:
		m.screen = screenJobsActions
	case m.screen == screenJobsActions:
		m.screen = screenJobs
		m.showOrgPicker = false // defensive: clear any stale picker state
	case m.screen == screenJobs:
		m.screen = screenMenu
	case m.screen == screenSSHKeys:
		m.screen = screenMenu
	}
	return m, nil
}

// moveCursor adjusts the relevant cursor by delta (+1 or -1) for the current screen/state.
func (m *model) moveCursor(delta int) {
	switch {
	case m.screen == screenMenu:
		m.menuCursor = clamp(m.menuCursor+delta, 0, 2)
	case m.screen == screenJobs && !m.showOrgPicker:
		if len(m.jobs) > 0 {
			m.jobCursor = clamp(m.jobCursor+delta, 0, len(m.jobs)-1)
		}
	case m.screen == screenJobsActions:
		m.actionCursor = clamp(m.actionCursor+delta, 0, 2)
	case m.screen == screenLogin:
		m.loginCursor = clamp(m.loginCursor+delta, 0, 1)
	case m.showOrgPicker && len(m.githubRepos) > 0:
		m.githubRepoCursor = clamp(m.githubRepoCursor+delta, 0, len(m.githubRepos)-1)
	case m.showOrgPicker && len(m.repositories) == 0 && len(m.organizations) > 0:
		m.orgCursor = clamp(m.orgCursor+delta, 0, len(m.organizations)-1)
	case m.showOrgPicker && len(m.repositories) > 0:
		m.repoCursor = clamp(m.repoCursor+delta, 0, len(m.repositories)-1)
	}
}

// clamp returns v clamped to [lo, hi]. When the list is empty hi may be -1;
// the len > 0 guard in moveCursor prevents reaching clamp in that case, but
// the hi < lo check here is a safety net for any future call site.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
