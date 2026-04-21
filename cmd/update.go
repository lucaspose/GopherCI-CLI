package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

const (
	defaultJobsPageSize  = 15
	defaultReposPageSize = 12
	minPageSize          = 3
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		next, cmd, handled := m.handleKeyWithStatus(msg)
		updated := next.(model)
		if handled {
			return updated, cmd
		}
		return updated.updateActiveInput(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyResponsiveLayout()
		return m, nil

	case loginSuccessMsg:
		m.config.Token = msg.token
		config.Save(m.config)
		m.apiClient.Token = msg.token
		m.screen = screenMenu
		m.err = ""
		return m, nil

	case jobsLoadedMsg:
		preserveView := !m.loading
		prevCursor := m.jobs.cursor
		prevOffset := m.jobs.pageOffset
		prevSelectedID := ""
		if m.selectedJob != nil {
			prevSelectedID = m.selectedJob.ID
		}

		m.jobs.jobs = msg.jobs
		if preserveView {
			m.restoreJobsView(prevCursor, prevOffset, prevSelectedID)
		} else {
			m.jobs.cursor = 0
			m.jobs.pageOffset = 0
		}
		m.loading = false
		m.err = ""
		if cmd := m.startJobsStreamIfNeeded(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case jobsStreamOpenedMsg:
		if !m.wantsJobsStream() {
			if msg.stop != nil {
				msg.stop()
			}
			m.jobsStream = jobsStreamState{}
			return m, nil
		}

		if m.jobsStream.stop != nil {
			m.jobsStream.stop()
		}
		m.jobsStream.messages = msg.messages
		m.jobsStream.stop = msg.stop
		m.jobsStream.opening = false
		m.jobsStream.retryDelay = time.Second
		m.err = ""
		return m, waitNextJobsStreamMessage(msg.messages)

	case jobsStreamEventMsg:
		prevCursor := m.jobs.cursor
		prevOffset := m.jobs.pageOffset
		prevSelectedID := ""
		if m.selectedJob != nil {
			prevSelectedID = m.selectedJob.ID
		}

		if msg.full {
			m.jobs.jobs = m.filterStreamJobsForContext(msg.jobs)
		} else {
			m.jobs.jobs = m.mergeStreamJobs(msg.jobs)
		}
		m.restoreJobsView(prevCursor, prevOffset, prevSelectedID)
		m.err = ""

		if m.jobsStream.messages != nil {
			return m, waitNextJobsStreamMessage(m.jobsStream.messages)
		}
		return m, nil

	case jobsStreamFailedMsg:
		m.stopJobsStream()
		if strings.Contains(msg.err, "401") {
			m.config.Token = ""
			config.Save(m.config)
			m.screen = screenLogin
			m.loading = false
			m.err = ""
			return m, nil
		}
		m.err = "Mise à jour en direct interrompue. Reconnexion en cours..."
		if cmd := m.scheduleJobsStreamRetry(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case jobsStreamClosedMsg:
		m.stopJobsStream()
		if cmd := m.scheduleJobsStreamRetry(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case jobsStreamRetryTickMsg:
		if cmd := m.startJobsStreamIfNeeded(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case orgsLoadedMsg:
		m.repos.orgs = msg.orgs
		m.repos.orgCursor = 0
		m.repos.orgPageOffset = 0
		m.repos.subMode = 0
		m.loading = false
		m.err = ""
		return m, nil

	case reposLoadedMsg:
		m.repos.repos = msg.repos
		m.repos.repoCursor = 0
		m.repos.repoPageOffset = 0
		m.repos.subMode = 1
		m.loading = false
		m.err = ""
		return m, nil

	case jobDeletedMsg:
		m.actions.confirmDelete = false
		m.screen = screenJobs
		m.loading = true
		m.loadingMsg = "Actualisation des jobs..."
		return m, refreshJobs(m.apiClient, m)

	case jobCreatedMsg:
		m.screen = screenJobs
		m.loading = true
		m.loadingMsg = "Actualisation des jobs..."
		return m, refreshJobs(m.apiClient, m)

	case githubTokenMsg:
		m.config.GitHubToken = msg.token
		config.Save(m.config)
		return m, exchangeGitHubToken(m.apiClient, msg.token)

	case githubReposLoadedMsg:
		m.repos.githubRepos = msg.repos
		m.repos.githubCursor = 0
		m.repos.githubPageOffset = 0
		m.repos.subMode = 0
		m.loading = false
		m.err = ""
		return m, nil

	case sshKeysLoadedMsg:
		m.ssh.keys = msg.keys
		m.ssh.cursor = 0
		m.loading = false
		m.err = ""
		return m, nil

	case sshKeyCreatedMsg:
		m.ssh.mode = 0
		m.loading = true
		m.loadingMsg = "Actualisation des clés..."
		return m, fetchSSHKeys(m.apiClient)

	case sshKeyDeletedMsg:
		m.ssh.confirmDelete = false
		m.loading = true
		m.loadingMsg = "Actualisation des clés..."
		return m, fetchSSHKeys(m.apiClient)

	case clipboardCopiedMsg:
		m.successMsg = "Fingerprint copié dans le presse-papiers !"
		return m, nil

	case apiURLSavedMsg:
		m.successMsg = "URL de l'API sauvegardée !"
		m.settings.editing = false
		return m, nil

	case artifactDownloadedMsg:
		m.loading = false
		m.err = ""
		m.successMsg = "Artifact téléchargé: " + msg.path
		return m, nil

	case errMsg:
		rawErr := strings.TrimSpace(string(msg))
		m.err = presentableError(rawErr, m.screen)
		m.loading = false
		if shouldForceReauth(rawErr, m.screen) {
			m.stopJobsStream()
			m.config.Token = ""
			config.Save(m.config)
			m.screen = screenLogin
			m.loading = false
			m.err = ""
		}
		return m, nil

	case tickMsg:
		cmd := startAuthTicker()
		if m.screen != screenLogin && !m.loading {
			return m, tea.Batch(cmd, checkAuthCmd(m.apiClient))
		}
		return m, cmd
	}

	// Forward non-key messages (blink/tick/etc.) to the active input.
	return m.updateActiveInput(msg)
}

func (m model) updateActiveInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch {
	case m.screen == screenNewJob && m.newJob.mode == 1:
		var cmd tea.Cmd
		m.newJob.cmdInput, cmd = m.newJob.cmdInput.Update(msg)
		return m, cmd
	case m.screen == screenSSHKeys && m.ssh.mode == 1:
		var cmd tea.Cmd
		m.ssh.nameInput, cmd = m.ssh.nameInput.Update(msg)
		return m, cmd
	case m.screen == screenSSHKeys && m.ssh.mode == 2:
		var cmd tea.Cmd
		m.ssh.pubInput, cmd = m.ssh.pubInput.Update(msg)
		return m, cmd
	case m.screen == screenSettings && m.settings.editing:
		var cmd tea.Cmd
		m.settings.apiURLInput, cmd = m.settings.apiURLInput.Update(msg)
		return m, cmd
	case m.screen == screenLogin && len(m.login.inputs) > 0:
		m.login.focused = clamp(m.login.focused, 0, len(m.login.inputs)-1)
		var cmd tea.Cmd
		m.login.inputs[m.login.focused], cmd = m.login.inputs[m.login.focused].Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	next, cmd, _ := m.handleKeyWithStatus(msg)
	return next, cmd
}

func (m model) handleKeyWithStatus(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// Clear success message on any keypress.
	m.successMsg = ""

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true

	case "tab":
		if m.screen == screenLogin {
			m.login.inputs[m.login.focused].Blur()
			m.login.focused = (m.login.focused + 1) % len(m.login.inputs)
			m.login.inputs[m.login.focused].Focus()
			return m, nil, true
		}
		return m, nil, false

	case "enter":
		next, cmd := m.handleEnter()
		return next, cmd, true

	case "esc":
		next, cmd := m.handleEsc()
		return next, cmd, true

	case "up":
		m.moveCursor(-1)
		return m, nil, true

	case "down":
		m.moveCursor(+1)
		return m, nil, true

	case "left":
		if m.screen == screenRepos {
			m.moveReposPage(-1)
			return m, nil, true
		}
		return m, nil, false

	case "right":
		if m.screen == screenRepos {
			m.moveReposPage(+1)
			return m, nil, true
		}
		return m, nil, false

	case "pgdown", "ctrl+f":
		if m.screen == screenJobs {
			visible := filterJobs(m.jobs.jobs, m.jobs.filter)
			_, total := paginateJobs(visible, m.jobs.pageSize, m.jobs.pageOffset)
			currentPage := m.jobs.pageOffset/m.jobs.pageSize + 1
			if currentPage < total {
				m.jobs.pageOffset += m.jobs.pageSize
				m.jobs.cursor = m.jobs.pageOffset
			}
			return m, nil, true
		}
		return m, nil, false

	case "pgup", "ctrl+b":
		if m.screen == screenJobs {
			if m.jobs.pageOffset > 0 {
				m.jobs.pageOffset -= m.jobs.pageSize
				if m.jobs.pageOffset < 0 {
					m.jobs.pageOffset = 0
				}
				m.jobs.cursor = m.jobs.pageOffset
			}
			return m, nil, true
		}
		return m, nil, false

	case "n":
		if m.screen == screenJobs {
			if m.hasRepoContext() {
				m = initNewJobState(m)
				m.screen = screenNewJob
				return m, nil, true
			}
			m.screen = screenJobs
			m.loading = true
			m.loadingMsg = "Chargement des dépôts..."
			return m, fetchOrgsOrGitHub(m.apiClient, m.config), true
		}
		return m, nil, false

	case "f":
		if m.screen == screenJobs {
			statuses := []string{"", "running", "failed", "success", "pending"}
			cur := m.jobs.filter
			next := ""
			for i, s := range statuses {
				if s == cur {
					next = statuses[(i+1)%len(statuses)]
					break
				}
			}
			m.jobs.filter = next
			m.jobs.pageOffset = 0
			m.jobs.cursor = 0
			return m, nil, true
		}
		return m, nil, false

	case "a":
		if m.screen == screenSSHKeys && m.ssh.mode == 0 {
			next, cmd := m.startAddSSHKey()
			return next, cmd, true
		}
		return m, nil, false

	case "d":
		if m.screen == screenSSHKeys && m.ssh.mode == 0 && len(m.ssh.keys) > 0 {
			m.ssh.confirmDelete = true
			return m, nil, true
		}
		return m, nil, false

	case "c":
		if m.screen == screenSSHKeys && m.ssh.mode == 0 && len(m.ssh.keys) > 0 {
			key := m.ssh.keys[m.ssh.cursor]
			fp := key.Fingerprint
			if fp == "" {
				fp = key.PublicKey
			}
			return m, copyToClipboard(fp), true
		}
		return m, nil, false

	case "y":
		// Confirm pending delete.
		if m.screen == screenSSHKeys && m.ssh.confirmDelete && len(m.ssh.keys) > 0 {
			key := m.ssh.keys[m.ssh.cursor]
			m.ssh.confirmDelete = false
			m.loading = true
			m.loadingMsg = "Suppression de la clé..."
			return m, deleteSSHKey(m.apiClient, key.ID), true
		}
		if m.screen == screenJobsActions && m.actions.confirmDelete && m.selectedJob != nil {
			m.actions.confirmDelete = false
			m.loading = true
			m.loadingMsg = "Suppression du job..."
			return m, dispatchDelete(m.apiClient, m), true
		}
		return m, nil, false

	case "j":
		if m.screen == screenRepos {
			next, cmd := m.handleReposJumpToJobs()
			return next, cmd, true
		}
		return m, nil, false

	case "e":
		if m.screen == screenSettings && !m.settings.editing {
			input := textinput.New()
			input.Placeholder = "http://localhost:8080"
			input.SetValue(m.config.APIURL)
			input.Focus()
			m.settings.apiURLInput = input
			m.settings.editing = true
			return m, nil, true
		}
		return m, nil, false
	}

	return m, nil, false
}

// hasRepoContext reports whether a repository is already selected.
func (m model) hasRepoContext() bool {
	return m.repos.selectedGH != nil ||
		(m.repos.selectedOrg != nil && m.repos.selectedRepo != nil)
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenLogin:
		if m.login.cursor == 1 {
			m.err = ""
			m.successMsg = "Connexion GitHub: ouverture du navigateur..."
			return m, waitForGitHubToken(m.config.APIURL)
		}
		return m, doLogin(m.apiClient, m.login.inputs[0].Value(), m.login.inputs[1].Value())

	case screenMenu:
		return m.handleMenuEnter()

	case screenRepos:
		return m.handleReposEnter()

	case screenJobs:
		if len(m.jobs.jobs) > 0 {
			visible := filterJobs(m.jobs.jobs, m.jobs.filter)
			page, _ := paginateJobs(visible, m.jobs.pageSize, m.jobs.pageOffset)
			// cursor is relative to the full jobs list; find actual index in page
			pageIdx := m.jobs.cursor - m.jobs.pageOffset
			if pageIdx >= 0 && pageIdx < len(page) {
				m.selectedJob = &page[pageIdx]
				m.screen = screenJobsActions
				m.actions.cursor = 0
				m.actions.confirmDelete = false
			}
		}
		return m, nil

	case screenJobsActions:
		return m.handleActionEnter()

	case screenNewJob:
		return m.handleNewJobEnter()

	case screenSSHKeys:
		return m.handleSSHKeyEnter()

	case screenSettings:
		if m.settings.editing {
			newURL := strings.TrimSpace(m.settings.apiURLInput.Value())
			if newURL != "" {
				m.config.APIURL = newURL
				config.Save(m.config)
				m.apiClient = newAPIClient(m.config)
			}
			m.settings.editing = false
			m.settings.apiURLInput.Blur()
			m.successMsg = "URL de l'API sauvegardée !"
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleMenuEnter() (tea.Model, tea.Cmd) {
	switch m.menuCursor {
	case 0: // Repositories
		m.screen = screenRepos
		m.repos = reposState{} // reset
		m.loading = true
		m.loadingMsg = "Chargement des dépôts..."
		return m, fetchOrgsOrGitHub(m.apiClient, m.config)
	case 1: // Jobs
		m.screen = screenJobs
		m.loading = true
		m.loadingMsg = "Chargement des dépôts..."
		return m, fetchOrgsOrGitHub(m.apiClient, m.config)
	case 2: // SSH Keys
		m = initSSHKeyState(m)
		m.screen = screenSSHKeys
		m.loading = true
		m.loadingMsg = "Chargement des clés SSH..."
		return m, fetchSSHKeys(m.apiClient)
	case 3: // Settings
		m.screen = screenSettings
		m.settings.editing = false
		return m, nil
	case 4: // Help
		m.screen = screenHelp
		return m, nil
	case 5: // Logout
		m.stopJobsStream()
		m.config.Token = ""
		m.config.GitHubToken = ""
		config.Save(m.config)
		m.screen = screenLogin
		m.err = ""
	}
	return m, nil
}

func (m model) handleReposEnter() (tea.Model, tea.Cmd) {
	// GitHub flow
	if len(m.repos.githubRepos) > 0 {
		if m.repos.githubCursor >= 0 && m.repos.githubCursor < len(m.repos.githubRepos) {
			selected := m.repos.githubRepos[m.repos.githubCursor]
			m.repos.selectedGH = &selected
		}
		return m, nil
	}
	// Org flow: subMode 0 = select org, subMode 1 = select repo
	if m.repos.subMode == 0 {
		if m.repos.orgCursor < 0 || m.repos.orgCursor >= len(m.repos.orgs) {
			return m, nil
		}
		m.repos.selectedOrg = &m.repos.orgs[m.repos.orgCursor]
		m.repos.selectedRepo = nil
		m.repos.repos = nil
		m.repos.repoCursor = 0
		m.repos.repoPageOffset = 0
		m.loading = true
		m.loadingMsg = "Chargement des dépôts..."
		return m, fetchRepos(m.apiClient, m.repos.selectedOrg.ID)
	}
	// subMode 1 — select repo
	if m.repos.repoCursor >= 0 && m.repos.repoCursor < len(m.repos.repos) {
		m.repos.selectedRepo = &m.repos.repos[m.repos.repoCursor]
	}
	return m, nil
}

func (m model) handleReposJumpToJobs() (tea.Model, tea.Cmd) {
	if !m.hasRepoContext() {
		return m, nil
	}
	m.screen = screenJobs
	m.jobs = jobsState{pageSize: m.jobsPageSize()}
	m.loading = true
	m.loadingMsg = "Chargement des jobs..."
	return m, refreshJobs(m.apiClient, m)
}

func (m model) handleActionEnter() (tea.Model, tea.Cmd) {
	switch m.actions.cursor {
	case 0: // View Logs
		m.screen = screenLogs
		m.logs.offset = 0
	case 1: // Re-run
		if m.repos.selectedGH != nil {
			cloneURL := preferredGitHubCloneURL(m.repos.selectedGH)
			if cloneURL == "" && m.selectedJob != nil {
				cloneURL = strings.TrimSpace(m.selectedJob.CloneURL)
			}
			if cloneURL == "" {
				m.err = "aucune URL de clone pour le re-run"
				return m, nil
			}
			return m, rerunJobWithURL(m.apiClient, cloneURL)
		}
		if m.repos.selectedOrg != nil && m.repos.selectedRepo != nil {
			return m, rerunJob(m.apiClient, m.repos.selectedOrg.ID, m.repos.selectedRepo.ID)
		}
		m.err = "aucun contexte de dépôt pour le re-run"
		return m, nil
	case 2: // Download artifact ZIP
		if m.selectedJob == nil || strings.TrimSpace(m.selectedJob.ID) == "" {
			m.err = "job invalide"
			return m, nil
		}
		m.loading = true
		m.loadingMsg = "Téléchargement de l'artifact..."
		return m, downloadJobArtifact(m.apiClient, m.selectedJob.ID)
	case 3: // Delete
		m.actions.confirmDelete = true
		return m, nil
	}
	return m, nil
}

func (m model) handleNewJobEnter() (tea.Model, tea.Cmd) {
	switch m.newJob.mode {
	case 0:
		switch m.newJob.cursor {
		case 0:
			if !m.newJob.hasGoci {
				return m, nil
			}
			m.loading = true
			m.loadingMsg = "Création du job..."
			return m, createJobFromGoci(m.apiClient, m)
		case 1:
			m.newJob.mode = 1
			m.newJob.cmdInput.Focus()
			return m, nil
		}
	case 1:
		cmd := strings.TrimSpace(m.newJob.cmdInput.Value())
		if cmd == "" {
			return m, nil
		}
		m.loading = true
		m.loadingMsg = "Création du job..."
		return m, createJobFromCmd(m.apiClient, m, cmd)
	}
	return m, nil
}

func (m model) handleSSHKeyEnter() (tea.Model, tea.Cmd) {
	switch m.ssh.mode {
	case 0:
		return m, nil
	case 1:
		name := strings.TrimSpace(m.ssh.nameInput.Value())
		if name == "" {
			return m, nil
		}
		m.ssh.newName = name
		m.ssh.mode = 2
		m.ssh.nameInput.Blur()
		m.ssh.pubInput.Focus()
		return m, nil
	case 2:
		privateKey, err := normalizeSSHPrivateKey(m.ssh.pubInput.Value())
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.ssh.pubInput.SetValue(privateKey)
		m.loading = true
		m.loadingMsg = "Ajout de la clé..."
		return m, createSSHKey(m.apiClient, m.ssh.newName, privateKey)
	}
	return m, nil
}

// normalizeSSHPrivateKey accepts either a private key content or a path to a private key file.
func normalizeSSHPrivateKey(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("clé SSH invalide: fournissez une clé privée ou un chemin de fichier")
	}

	if isLikelyPath(value) {
		fromFile, err := readSSHKeyFile(value)
		if err != nil {
			return "", err
		}
		value = fromFile
	}

	if looksLikePublicSSHKey(value) {
		return "", fmt.Errorf("clé SSH invalide: collez une clé privée (~/.ssh/id_ed25519), pas la clé .pub")
	}

	return value, nil
}

func looksLikePublicSSHKey(value string) bool {
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return false
	}

	keyType := parts[0]
	if keyType == "ssh-rsa" ||
		strings.HasPrefix(keyType, "ssh-ed25519") ||
		strings.HasPrefix(keyType, "ecdsa-sha2-") ||
		strings.HasPrefix(keyType, "sk-") {
		return true
	}

	return false
}

func isLikelyPath(value string) bool {
	return strings.HasPrefix(value, "~/") ||
		strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../")
}

func readSSHKeyFile(path string) (string, error) {
	resolved := path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("impossible de résoudre le home: %w", err)
		}
		resolved = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("impossible de lire la clé privée (%s)", resolved)
	}

	value := strings.TrimSpace(string(content))
	if value == "" {
		return "", fmt.Errorf("fichier de clé privée vide (%s)", resolved)
	}

	return value, nil
}

func (m model) startAddSSHKey() (model, tea.Cmd) {
	m = initSSHKeyAddState(m)
	m.ssh.mode = 1
	m.ssh.nameInput.Focus()
	return m, nil
}

func (m model) handleEsc() (tea.Model, tea.Cmd) {
	m.err = ""
	m.successMsg = ""

	// Cancel pending confirmations first.
	if m.ssh.confirmDelete {
		m.ssh.confirmDelete = false
		return m, nil
	}
	if m.actions.confirmDelete {
		m.actions.confirmDelete = false
		return m, nil
	}

	switch {
	case m.screen == screenSettings && m.settings.editing:
		m.settings.editing = false
		m.settings.apiURLInput.Blur()
	case m.screen == screenSettings:
		m.screen = screenMenu
	case m.screen == screenHelp:
		m.screen = screenMenu
	case m.screen == screenSSHKeys && m.ssh.mode == 2:
		m.ssh.mode = 1
		m.ssh.pubInput.Blur()
		m.ssh.nameInput.Focus()
	case m.screen == screenSSHKeys && m.ssh.mode == 1:
		m.ssh.mode = 0
		m.ssh.nameInput.Blur()
	case m.screen == screenSSHKeys:
		m.screen = screenMenu
	case m.screen == screenNewJob && m.newJob.mode == 1:
		m.newJob.mode = 0
		m.newJob.cmdInput.Blur()
	case m.screen == screenNewJob:
		m.screen = screenJobs
	case m.screen == screenLogs:
		m.screen = screenJobsActions
	case m.screen == screenJobsActions:
		m.screen = screenJobs
	case m.screen == screenJobs:
		m.stopJobsStream()
		// If we came from repos, go back to repos; otherwise menu.
		if m.repos.selectedOrg != nil || m.repos.selectedGH != nil {
			m.screen = screenRepos
		} else {
			m.screen = screenMenu
		}
	case m.screen == screenRepos && m.repos.subMode == 1:
		m.repos.subMode = 0
		m.repos.selectedRepo = nil
		m.repos.repos = nil
		m.repos.repoCursor = 0
		m.repos.repoPageOffset = 0
	case m.screen == screenRepos:
		m.screen = screenMenu
	}
	return m, nil
}

func (m model) wantsJobsStream() bool {
	if m.loading {
		return false
	}
	if m.apiClient == nil || strings.TrimSpace(m.apiClient.Token) == "" {
		return false
	}
	if m.screen != screenJobs && m.screen != screenJobsActions && m.screen != screenLogs {
		return false
	}
	return m.hasRepoContext()
}

func (m *model) stopJobsStream() {
	if m.jobsStream.stop != nil {
		m.jobsStream.stop()
	}
	m.jobsStream = jobsStreamState{}
}

func (m *model) startJobsStreamIfNeeded() tea.Cmd {
	if !m.wantsJobsStream() {
		m.stopJobsStream()
		return nil
	}
	if m.jobsStream.messages != nil || m.jobsStream.opening {
		return nil
	}
	m.jobsStream.opening = true
	return openJobsStream(m.apiClient, *m)
}

func (m *model) scheduleJobsStreamRetry() tea.Cmd {
	if !m.wantsJobsStream() {
		return nil
	}
	if m.jobsStream.messages != nil || m.jobsStream.opening {
		return nil
	}
	if m.jobsStream.retryDelay <= 0 {
		m.jobsStream.retryDelay = time.Second
	} else {
		m.jobsStream.retryDelay *= 2
		if m.jobsStream.retryDelay > 30*time.Second {
			m.jobsStream.retryDelay = 30 * time.Second
		}
	}
	return startJobsStreamRetryTicker(m.jobsStream.retryDelay)
}

func (m model) filterStreamJobsForContext(jobs []api.Job) []api.Job {
	if m.repos.selectedGH == nil {
		return jobs
	}

	targets := make(map[string]struct{})
	for _, raw := range githubCloneURLCandidates(m.repos.selectedGH) {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			targets[trimmed] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return nil
	}

	filtered := make([]api.Job, 0, len(jobs))
	for _, job := range jobs {
		if _, ok := targets[job.CloneURL]; ok {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func (m model) mergeStreamJobs(updates []api.Job) []api.Job {
	if len(updates) == 0 {
		return m.jobs.jobs
	}

	filtered := m.filterStreamJobsForContext(updates)
	if len(filtered) == 0 {
		return m.jobs.jobs
	}

	out := append([]api.Job(nil), m.jobs.jobs...)
	indexByID := make(map[string]int, len(out))
	for i, job := range out {
		indexByID[job.ID] = i
	}

	for _, job := range filtered {
		if idx, ok := indexByID[job.ID]; ok {
			out[idx] = job
			continue
		}
		out = append([]api.Job{job}, out...)
		for id, idx := range indexByID {
			indexByID[id] = idx + 1
		}
		indexByID[job.ID] = 0
	}

	return out
}

func (m *model) restoreJobsView(prevCursor, prevOffset int, prevSelectedID string) {
	visible := filterJobs(m.jobs.jobs, m.jobs.filter)
	if len(visible) == 0 {
		m.jobs.cursor = 0
		m.jobs.pageOffset = 0
		m.selectedJob = nil
		return
	}

	target := clamp(prevCursor, 0, len(visible)-1)
	if prevSelectedID != "" {
		for i, job := range visible {
			if job.ID == prevSelectedID {
				target = i
				break
			}
		}
	}

	pageSize := m.jobs.pageSize
	if pageSize <= 0 {
		pageSize = len(visible)
	}

	maxOffset := ((len(visible) - 1) / pageSize) * pageSize
	offset := clamp(prevOffset, 0, maxOffset)
	if target < offset || target >= offset+pageSize {
		offset = (target / pageSize) * pageSize
	}

	m.jobs.cursor = target
	m.jobs.pageOffset = offset

	if prevSelectedID == "" {
		return
	}

	m.selectedJob = nil
	for i := range m.jobs.jobs {
		if m.jobs.jobs[i].ID == prevSelectedID {
			m.selectedJob = &m.jobs.jobs[i]
			break
		}
	}
}

func (m model) jobsPageSize() int {
	if m.height <= 0 {
		return defaultJobsPageSize
	}
	size := m.height - 12
	if size < minPageSize {
		return minPageSize
	}
	return size
}

func (m model) reposPageSize() int {
	if m.height <= 0 {
		return defaultReposPageSize
	}
	size := m.height - 11
	if size < minPageSize {
		return minPageSize
	}
	return size
}

func (m model) githubReposRowBudget() int {
	if m.height <= 0 {
		return defaultReposPageSize
	}
	// GitHub list includes owner headers; keep extra room for footer/pagination.
	size := m.height - 15
	if size < minPageSize {
		return minPageSize
	}
	return size
}

func githubRepoOwner(fullName string) string {
	parts := strings.SplitN(fullName, "/", 2)
	owner := strings.TrimSpace(parts[0])
	if owner == "" {
		owner = strings.TrimSpace(fullName)
	}
	if owner == "" {
		return "unknown"
	}
	return owner
}

func githubRepoPageStarts(repos []api.GitHubRepo, rowBudget int) []int {
	if len(repos) == 0 {
		return nil
	}
	if rowBudget <= 0 {
		rowBudget = len(repos)
	}

	starts := []int{0}
	for i := 0; i < len(repos); {
		rows := 0
		ownerOnPage := ""

		for i < len(repos) {
			owner := githubRepoOwner(repos[i].FullName)
			needed := 1 // repository row
			if owner != ownerOnPage {
				needed++ // owner header row
			}

			if rows > 0 && rows+needed > rowBudget {
				break
			}

			rows += needed
			ownerOnPage = owner
			i++
		}

		if i < len(repos) {
			starts = append(starts, i)
		}
	}

	return starts
}

func githubRepoPageIndex(starts []int, offset int) int {
	if len(starts) == 0 {
		return 0
	}
	idx := 0
	for i := 1; i < len(starts); i++ {
		if starts[i] > offset {
			break
		}
		idx = i
	}
	return idx
}

func githubRepoPageBounds(repos []api.GitHubRepo, rowBudget, offset int) (start, end, totalPages, currentPage, normalizedOffset int) {
	if len(repos) == 0 {
		return 0, 0, 1, 1, 0
	}
	if rowBudget <= 0 {
		rowBudget = len(repos)
	}

	starts := githubRepoPageStarts(repos, rowBudget)
	totalPages = len(starts)
	idx := githubRepoPageIndex(starts, offset)

	start = starts[idx]
	normalizedOffset = start
	if idx+1 < len(starts) {
		end = starts[idx+1]
	} else {
		end = len(repos)
	}
	currentPage = idx + 1
	return
}

func moveGitHubRepoPage(repos []api.GitHubRepo, rowBudget, offset, cursor, delta int) (int, int) {
	if len(repos) == 0 {
		return 0, 0
	}

	start, end, totalPages, currentPage, normalizedOffset := githubRepoPageBounds(repos, rowBudget, offset)
	cursor = clampCursorToPage(cursor, start, end)
	if delta == 0 {
		return normalizedOffset, cursor
	}

	targetPage := clamp(currentPage+delta, 1, totalPages)
	if targetPage == currentPage {
		return normalizedOffset, cursor
	}

	starts := githubRepoPageStarts(repos, rowBudget)
	newStart := starts[targetPage-1]
	newEnd := len(repos)
	if targetPage < totalPages {
		newEnd = starts[targetPage]
	}

	relative := cursor - start
	if relative < 0 {
		relative = 0
	}
	newCursor := newStart + relative
	if newCursor >= newEnd {
		newCursor = newEnd - 1
	}

	return newStart, newCursor
}

func clampCursorToPage(cursor, start, end int) int {
	if end <= start {
		return 0
	}
	if cursor < start || cursor >= end {
		return start
	}
	return clamp(cursor, start, end-1)
}

func moveListPage(offset, cursor, total, pageSize, delta int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if pageSize <= 0 {
		pageSize = total
	}

	start, end, _, currentOffset := paginateRange(total, pageSize, offset)
	cursor = clampCursorToPage(cursor, start, end)

	targetOffset := currentOffset + (delta * pageSize)
	newStart, newEnd, _, newOffset := paginateRange(total, pageSize, targetOffset)
	if newOffset == currentOffset {
		return currentOffset, cursor
	}

	relative := cursor - start
	if relative < 0 {
		relative = 0
	}

	newCursor := newStart + relative
	if newCursor >= newEnd {
		newCursor = newEnd - 1
	}

	return newOffset, newCursor
}

func (m *model) normalizeReposPagination() {
	pageSize := m.reposPageSize()

	switch {
	case len(m.repos.githubRepos) > 0:
		start, end, _, _, offset := githubRepoPageBounds(
			m.repos.githubRepos,
			m.githubReposRowBudget(),
			m.repos.githubPageOffset,
		)
		m.repos.githubPageOffset = offset
		if len(m.repos.githubRepos) == 0 {
			m.repos.githubCursor = 0
			return
		}
		m.repos.githubCursor = clampCursorToPage(m.repos.githubCursor, start, end)

	case m.repos.subMode == 0:
		start, end, _, offset := paginateRange(len(m.repos.orgs), pageSize, m.repos.orgPageOffset)
		m.repos.orgPageOffset = offset
		if len(m.repos.orgs) == 0 {
			m.repos.orgCursor = 0
			return
		}
		m.repos.orgCursor = clampCursorToPage(m.repos.orgCursor, start, end)

	case m.repos.subMode == 1:
		start, end, _, offset := paginateRange(len(m.repos.repos), pageSize, m.repos.repoPageOffset)
		m.repos.repoPageOffset = offset
		if len(m.repos.repos) == 0 {
			m.repos.repoCursor = 0
			return
		}
		m.repos.repoCursor = clampCursorToPage(m.repos.repoCursor, start, end)
	}
}

func (m *model) moveReposPage(delta int) {
	if delta == 0 {
		return
	}

	pageSize := m.reposPageSize()
	switch {
	case len(m.repos.githubRepos) > 0:
		m.repos.githubPageOffset, m.repos.githubCursor = moveGitHubRepoPage(
			m.repos.githubRepos,
			m.githubReposRowBudget(),
			m.repos.githubPageOffset,
			m.repos.githubCursor,
			delta,
		)
	case m.repos.subMode == 0:
		m.repos.orgPageOffset, m.repos.orgCursor = moveListPage(
			m.repos.orgPageOffset,
			m.repos.orgCursor,
			len(m.repos.orgs),
			pageSize,
			delta,
		)
	case m.repos.subMode == 1:
		m.repos.repoPageOffset, m.repos.repoCursor = moveListPage(
			m.repos.repoPageOffset,
			m.repos.repoCursor,
			len(m.repos.repos),
			pageSize,
			delta,
		)
	}
}

func (m *model) applyResponsiveLayout() {
	prevCursor := m.jobs.cursor
	prevOffset := m.jobs.pageOffset
	prevSelectedID := ""
	if m.selectedJob != nil {
		prevSelectedID = m.selectedJob.ID
	}

	m.jobs.pageSize = m.jobsPageSize()
	if len(m.jobs.jobs) > 0 || m.screen == screenJobs {
		m.restoreJobsView(prevCursor, prevOffset, prevSelectedID)
	}
	m.normalizeReposPagination()
}

func (m *model) moveCursor(delta int) {
	switch {
	case m.screen == screenMenu:
		m.menuCursor = clamp(m.menuCursor+delta, 0, 5)
	case m.screen == screenJobs:
		visible := filterJobs(m.jobs.jobs, m.jobs.filter)
		page, _ := paginateJobs(visible, m.jobs.pageSize, m.jobs.pageOffset)
		if len(page) > 0 {
			pageIdx := (m.jobs.cursor - m.jobs.pageOffset) + delta
			pageIdx = clamp(pageIdx, 0, len(page)-1)
			m.jobs.cursor = m.jobs.pageOffset + pageIdx
		}
	case m.screen == screenJobsActions:
		m.actions.cursor = clamp(m.actions.cursor+delta, 0, 3)
	case m.screen == screenLogin:
		m.login.cursor = clamp(m.login.cursor+delta, 0, 1)
	case m.screen == screenNewJob && m.newJob.mode == 0:
		m.newJob.cursor = clamp(m.newJob.cursor+delta, 0, 1)
	case m.screen == screenSSHKeys && m.ssh.mode == 0 && len(m.ssh.keys) > 0:
		m.ssh.cursor = clamp(m.ssh.cursor+delta, 0, len(m.ssh.keys)-1)
	case m.screen == screenRepos:
		pageSize := m.reposPageSize()
		if len(m.repos.githubRepos) > 0 {
			start, end, _, _, offset := githubRepoPageBounds(
				m.repos.githubRepos,
				m.githubReposRowBudget(),
				m.repos.githubPageOffset,
			)
			m.repos.githubPageOffset = offset
			if end > start {
				m.repos.githubCursor = clampCursorToPage(m.repos.githubCursor, start, end)
				idx := (m.repos.githubCursor - start) + delta
				idx = clamp(idx, 0, end-start-1)
				m.repos.githubCursor = start + idx
			}
		} else if m.repos.subMode == 0 && len(m.repos.orgs) > 0 {
			start, end, _, offset := paginateRange(len(m.repos.orgs), pageSize, m.repos.orgPageOffset)
			m.repos.orgPageOffset = offset
			if end > start {
				m.repos.orgCursor = clampCursorToPage(m.repos.orgCursor, start, end)
				idx := (m.repos.orgCursor - start) + delta
				idx = clamp(idx, 0, end-start-1)
				m.repos.orgCursor = start + idx
			}
		} else if m.repos.subMode == 1 && len(m.repos.repos) > 0 {
			start, end, _, offset := paginateRange(len(m.repos.repos), pageSize, m.repos.repoPageOffset)
			m.repos.repoPageOffset = offset
			if end > start {
				m.repos.repoCursor = clampCursorToPage(m.repos.repoCursor, start, end)
				idx := (m.repos.repoCursor - start) + delta
				idx = clamp(idx, 0, end-start-1)
				m.repos.repoCursor = start + idx
			}
		}
	}
}

// ── Screen init helpers ───────────────────────────────────────────────────────

func initNewJobState(m model) model {
	steps, name, err := loadSteps()
	if err == nil {
		m.newJob.hasGoci = true
		m.newJob.steps = steps
		m.newJob.pipeline = name
	} else {
		m.newJob.hasGoci = false
		m.newJob.steps = nil
		m.newJob.pipeline = ""
	}
	m.newJob.cursor = 0
	m.newJob.mode = 0
	input := textinput.New()
	input.Placeholder = "ex: make test"
	m.newJob.cmdInput = input
	return m
}

func initSSHKeyState(m model) model {
	m.ssh.keys = nil
	m.ssh.cursor = 0
	m.ssh.mode = 0
	m.ssh.newName = ""
	m.ssh.confirmDelete = false
	return m
}

func initSSHKeyAddState(m model) model {
	nameInput := textinput.New()
	nameInput.Placeholder = "ex: deploy-key"
	nameInput.CharLimit = 80
	m.ssh.nameInput = nameInput

	pubInput := textinput.New()
	pubInput.Placeholder = "~/.ssh/id_ed25519"
	pubInput.CharLimit = 0
	m.ssh.pubInput = pubInput
	m.ssh.newName = ""
	return m
}

// newAPIClient creates a fresh API client from the current config.
func newAPIClient(cfg *config.Config) *api.Client {
	return api.NewClient(cfg.APIURL, cfg.Token)
}

func shouldForceReauth(rawErr string, currentScreen screen) bool {
	if currentScreen == screenLogin {
		return false
	}
	return strings.Contains(strings.ToLower(rawErr), "401")
}

func presentableError(rawErr string, currentScreen screen) string {
	rawErr = strings.TrimSpace(rawErr)
	if rawErr == "" {
		return "Une action n'a pas pu être terminée. Réessayez."
	}

	lower := strings.ToLower(rawErr)

	if strings.Contains(lower, "no .goci file found") {
		return "Fichier .goci introuvable dans ce dossier."
	}
	if strings.Contains(lower, ".goci file has no steps defined") {
		return "Le fichier .goci ne contient aucune étape."
	}
	if strings.Contains(lower, "no repository context") {
		return "Sélectionnez un dépôt avant de continuer."
	}
	if strings.Contains(lower, "no clone url") {
		return "Impossible de déterminer l'URL du dépôt."
	}
	if strings.Contains(lower, "download artifact failed: 404") {
		return "Aucun artifact ZIP disponible pour ce job."
	}
	if strings.Contains(lower, "download artifact failed: empty artifact") {
		return "Artifact vide: rien à télécharger pour ce job."
	}
	if strings.Contains(lower, "download artifact failed") {
		return "Impossible de télécharger l'artifact ZIP pour le moment."
	}
	if strings.Contains(lower, "artifact save failed") || strings.Contains(lower, "save artifact failed") {
		return "ZIP reçu, mais impossible de l'enregistrer localement."
	}

	if currentScreen == screenLogin {
		switch {
		case strings.Contains(lower, "401"), strings.Contains(lower, "login failed"):
			return "Connexion refusée. Vérifiez vos identifiants."
		case containsAny(lower, "github", "callback", "token"):
			return "Connexion GitHub impossible pour le moment."
		case isNetworkError(lower):
			return "Connexion au serveur impossible. Vérifiez l'URL de l'API puis réessayez."
		default:
			return "Connexion impossible pour le moment. Réessayez."
		}
	}

	if strings.Contains(lower, "401") {
		return "Votre session a expiré. Merci de vous reconnecter."
	}
	if strings.Contains(lower, "429") {
		return "Le service est momentanément occupé. Réessayez dans quelques secondes."
	}
	if isServerError(lower) {
		return "Le service est temporairement indisponible. Réessayez dans un instant."
	}
	if isNetworkError(lower) {
		return "Serveur injoignable pour le moment. Vérifiez la connexion puis réessayez."
	}

	switch currentScreen {
	case screenJobs:
		return "Impossible de charger les jobs pour le moment."
	case screenJobsActions:
		return "Action impossible pour ce job pour le moment."
	case screenLogs:
		return "Impossible d'afficher les logs pour le moment."
	case screenRepos:
		return "Impossible de charger les dépôts pour le moment."
	case screenSSHKeys:
		return "L'action sur les clés SSH n'a pas abouti."
	case screenSettings:
		return "La mise à jour des paramètres a échoué."
	default:
		return "Une action n'a pas pu être terminée. Réessayez."
	}
}

func containsAny(s string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

func isNetworkError(lower string) bool {
	return containsAny(lower,
		"dial tcp",
		"connection refused",
		"no such host",
		"timeout",
		"deadline exceeded",
		"network is unreachable",
		"temporary failure",
		"connection reset",
		"broken pipe",
		"eof",
	)
}

func isServerError(lower string) bool {
	return containsAny(lower, "500", "502", "503", "504")
}

// ── clamp ─────────────────────────────────────────────────────────────────────

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
