package main

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

type screen int

const (
	screenLogin screen = iota
	screenMenu
	screenRepos
	screenJobs
	screenJobsActions
	screenLogs
	screenSSHKeys
	screenNewJob
	screenSettings
	screenHelp
)

// Message types returned by async commands.
type (
	loginSuccessMsg     struct{ token string }
	errMsg              string
	jobsLoadedMsg       struct{ jobs []api.Job }
	jobsStreamOpenedMsg struct {
		messages <-chan tea.Msg
		stop     func()
	}
	jobsStreamEventMsg struct {
		jobs []api.Job
		full bool
	}
	jobsStreamFailedMsg    struct{ err string }
	jobsStreamClosedMsg    struct{}
	jobsStreamRetryTickMsg struct{}
	orgsLoadedMsg          struct{ orgs []api.Organization }
	reposLoadedMsg         struct{ repos []api.Repository }
	jobDeletedMsg          struct{}
	jobCreatedMsg          struct{}
	githubTokenMsg         struct{ token string }
	githubReposLoadedMsg   struct{ repos []api.GitHubRepo }
	tickMsg                struct{}
	sshKeysLoadedMsg       struct{ keys []api.SSHKey }
	sshKeyCreatedMsg       struct{}
	sshKeyDeletedMsg       struct{}
	clipboardCopiedMsg     struct{}
	apiURLSavedMsg         struct{}
	artifactDownloadedMsg  struct{ path string }
)

// ── Per-screen state structs ──────────────────────────────────────────────────

type loginState struct {
	inputs  []textinput.Model
	focused int
	cursor  int // 0 = email/password, 1 = github
}

type jobsState struct {
	jobs       []api.Job
	cursor     int
	filter     string // "" | "running" | "failed" | "success" | "pending"
	pageOffset int
	pageSize   int
}

type actionsState struct {
	cursor        int
	confirmDelete bool
}

type logsState struct {
	offset int // scroll offset (line index)
}

type sshState struct {
	keys          []api.SSHKey
	cursor        int
	mode          int // 0=list, 1=add name, 2=add pubkey
	nameInput     textinput.Model
	pubInput      textinput.Model
	newName       string
	confirmDelete bool
}

type newJobState struct {
	cursor   int
	mode     int // 0=method select, 1=command input
	cmdInput textinput.Model
	hasGoci  bool
	steps    []api.Step
	pipeline string
}

type reposState struct {
	orgs             []api.Organization
	orgCursor        int
	orgPageOffset    int
	selectedOrg      *api.Organization
	repos            []api.Repository
	repoCursor       int
	repoPageOffset   int
	selectedRepo     *api.Repository
	githubRepos      []api.GitHubRepo
	githubCursor     int
	githubPageOffset int
	selectedGH       *api.GitHubRepo
	subMode          int // 0=orgs/github list, 1=repos list
}

type settingsState struct {
	apiURLInput textinput.Model
	editing     bool
}

type jobsStreamState struct {
	messages   <-chan tea.Msg
	stop       func()
	opening    bool
	retryDelay time.Duration
}

// ── Top-level model ───────────────────────────────────────────────────────────

type model struct {
	screen    screen
	apiClient *api.Client
	config    *config.Config

	err        string
	successMsg string
	loading    bool
	loadingMsg string
	menuCursor int
	width      int
	height     int

	// selectedJob is shared between jobs/actions/logs screens.
	selectedJob *api.Job

	login      loginState
	jobs       jobsState
	actions    actionsState
	logs       logsState
	ssh        sshState
	newJob     newJobState
	repos      reposState
	settings   settingsState
	jobsStream jobsStreamState
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.login.inputs[0].Focus(),
		startAuthTicker(),
	}
	if m.screen != screenLogin {
		cmds = append(cmds, checkAuthCmd(m.apiClient))
	}
	return tea.Batch(cmds...)
}
