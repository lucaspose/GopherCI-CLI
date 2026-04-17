package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

// loadSteps reads the local .goci config and converts steps to API format.
// Shared by rerunJob and rerunJobWithURL to avoid duplication.
func loadSteps() ([]api.Step, error) {
	cfg, err := config.LoadGociConfig()
	if err != nil {
		return nil, fmt.Errorf("no .goci file found")
	}
	steps := make([]api.Step, len(cfg.Steps))
	for i, s := range cfg.Steps {
		steps[i] = api.Step{Name: s.Name, Cmd: s.Cmd}
	}
	return steps, nil
}

// refreshJobs re-fetches the job list using whichever flow was used to load
// jobs initially (GitHub clone URL or org/repo IDs). When neither context is
// available it returns an empty list so the loading state is never stuck.
func refreshJobs(client *api.Client, m model) tea.Cmd {
	if m.selectedGitHubRepo != nil {
		return fetchAllJobs(client, m.selectedGitHubRepo.CloneURL)
	}
	if m.selectedOrg != nil && m.selectedRepo != nil {
		return fetchJobs(client, m.selectedOrg.ID, m.selectedRepo.ID)
	}
	// No context: clear loading state with an empty list instead of hanging.
	return func() tea.Msg { return jobsLoadedMsg{jobs: nil} }
}

// dispatchDelete calls the right delete endpoint depending on whether we have
// org/repo context (org-flow) or only a job ID (github-flow).
func dispatchDelete(client *api.Client, m model) tea.Cmd {
	if m.selectedOrg != nil && m.selectedRepo != nil {
		return deleteJob(client, m.selectedOrg.ID, m.selectedRepo.ID, m.selectedJob.ID)
	}
	return deleteJobByID(client, m.selectedJob.ID)
}

// --- Auth ---

func doLogin(client *api.Client, email, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Login(email, password)
		if err != nil {
			return errMsg(err.Error())
		}
		return loginSuccessMsg{token: resp.AccessToken}
	}
}

func waitForGitHubToken() tea.Cmd {
	return func() tea.Msg {
		tokenChan := make(chan string, 1)
		mux := http.NewServeMux()
		srv := &http.Server{Addr: ":9999", Handler: mux}
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			fmt.Fprintf(w, "Token received! You can return to the CLI.")
			tokenChan <- token
		})
		go srv.ListenAndServe()
		token := <-tokenChan
		srv.Close()
		return githubTokenMsg{token: token}
	}
}

func exchangeGitHubToken(client *api.Client, githubToken string) tea.Cmd {
	return func() tea.Msg {
		token, err := client.ExchangeGitHubToken(githubToken)
		if err != nil {
			return errMsg(err.Error())
		}
		return loginSuccessMsg{token: token}
	}
}

func openGitHubLogin() {
	exec.Command("xdg-open", "http://localhost:8080/auth/github").Start()
}

// startAuthTicker returns a one-shot command that fires tickMsg after 30s.
// The tickMsg handler in Update() re-arms it each time to keep the cycle going.
func startAuthTicker() tea.Cmd {
	return tea.Every(30*time.Second, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// checkAuthCmd pings the API with the current token. On 401 it returns errMsg
// which triggers the existing redirect-to-login logic in Update().
func checkAuthCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := client.CheckAuth(); err != nil {
			return errMsg(err.Error())
		}
		return nil
	}
}

// --- Jobs ---

func fetchJobs(client *api.Client, orgID, repoID string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetJobs(orgID, repoID)
		if err != nil {
			return errMsg(err.Error())
		}
		return jobsLoadedMsg{jobs: jobs}
	}
}

func fetchAllJobs(client *api.Client, cloneURL string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetAllJobs()
		if err != nil {
			return errMsg(err.Error())
		}
		var filtered []api.Job
		for _, j := range jobs {
			if j.CloneURL == cloneURL {
				filtered = append(filtered, j)
			}
		}
		return jobsLoadedMsg{jobs: filtered}
	}
}

func deleteJob(client *api.Client, orgID, repoID, jobID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteJob(orgID, repoID, jobID); err != nil {
			return errMsg(err.Error())
		}
		return jobDeletedMsg{}
	}
}

// deleteJobByID is used when org/repo context is unavailable (github-flow).
func deleteJobByID(client *api.Client, jobID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteJobByID(jobID); err != nil {
			return errMsg(err.Error())
		}
		return jobDeletedMsg{}
	}
}

func rerunJob(client *api.Client, orgID, repoID string) tea.Cmd {
	return func() tea.Msg {
		steps, err := loadSteps()
		if err != nil {
			return errMsg(err.Error())
		}
		if err = client.CreateJob(orgID, repoID, steps); err != nil {
			return errMsg(err.Error())
		}
		return jobCreatedMsg{}
	}
}

func rerunJobWithURL(client *api.Client, cloneURL string) tea.Cmd {
	return func() tea.Msg {
		steps, err := loadSteps()
		if err != nil {
			return errMsg(err.Error())
		}
		if err = client.CreateJobWithURL(cloneURL, steps); err != nil {
			return errMsg(err.Error())
		}
		return jobCreatedMsg{}
	}
}

// --- Orgs / Repos ---

func fetchOrgsOrGitHub(client *api.Client, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if cfg.GitHubToken != "" {
			repos, err := client.GetGitHubRepositories(cfg.GitHubToken)
			if err != nil {
				return errMsg(err.Error())
			}
			return githubReposLoadedMsg{repos: repos}
		}
		orgs, err := client.GetOrganizations()
		if err != nil {
			return errMsg(err.Error())
		}
		return orgsLoadedMsg{orgs: orgs}
	}
}

func fetchRepos(client *api.Client, orgID string) tea.Cmd {
	return func() tea.Msg {
		repos, err := client.GetRepositories(orgID)
		if err != nil {
			return errMsg(err.Error())
		}
		return reposLoadedMsg{repos: repos}
	}
}
