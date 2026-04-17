package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

type screen int

const (
	screenLogin screen = iota
	screenMenu
	screenJobs
	screenJobsActions
	screenLogs
	screenSSHKeys
)

// Message types returned by async commands.
type (
	loginSuccessMsg      struct{ token string }
	errMsg               string
	jobsLoadedMsg        struct{ jobs []api.Job }
	orgsLoadedMsg        struct{ orgs []api.Organization }
	reposLoadedMsg       struct{ repos []api.Repository }
	jobDeletedMsg        struct{}
	jobCreatedMsg        struct{}
	githubTokenMsg       struct{ token string }
	githubReposLoadedMsg struct{ repos []api.GitHubRepo }
	tickMsg              struct{} // periodic auth check
)

type model struct {
	screen    screen
	apiClient *api.Client
	config    *config.Config

	// Login form
	inputs      []textinput.Model
	focused     int
	loginCursor int

	// Error / loading feedback
	err        string
	loading    bool
	loadingMsg string

	// Menu
	menuCursor int

	// Jobs list
	jobs      []api.Job
	jobCursor int

	// Selected job and its action cursor
	selectedJob  *api.Job
	actionCursor int

	// Org / repo picker (org-flow)
	showOrgPicker bool
	organizations []api.Organization
	orgCursor     int
	selectedOrg   *api.Organization
	repositories  []api.Repository
	repoCursor    int
	selectedRepo  *api.Repository

	// GitHub repo picker (github-flow)
	githubRepos        []api.GitHubRepo
	githubRepoCursor   int
	selectedGitHubRepo *api.GitHubRepo
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.inputs[0].Focus(), // starts cursor blink for the email input
		startAuthTicker(),
	)
}
