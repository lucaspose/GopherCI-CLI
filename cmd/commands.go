package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

const (
	githubOAuthCallbackAddr = "127.0.0.1:9999"
	githubOAuthTimeout      = 2 * time.Minute
)

// loadSteps reads the local .goci config and converts steps to API format.
func loadSteps() ([]api.Step, string, error) {
	cfg, err := config.LoadGociConfig()
	if err != nil {
		return nil, "", fmt.Errorf("no .goci file found in current directory")
	}
	if len(cfg.Steps) == 0 {
		return nil, cfg.Pipeline.Name, fmt.Errorf(".goci file has no steps defined")
	}
	steps := make([]api.Step, len(cfg.Steps))
	for i, s := range cfg.Steps {
		steps[i] = api.Step{Name: s.Name, Cmd: s.Cmd}
	}
	return steps, cfg.Pipeline.Name, nil
}

// refreshJobs re-fetches the job list using whichever flow was used to load jobs initially.
func refreshJobs(client *api.Client, m model) tea.Cmd {
	if m.repos.selectedGH != nil {
		return fetchAllJobs(client, githubCloneURLCandidates(m.repos.selectedGH)...)
	}
	if m.repos.selectedOrg != nil && m.repos.selectedRepo != nil {
		return fetchJobs(client, m.repos.selectedOrg.ID, m.repos.selectedRepo.ID)
	}
	return func() tea.Msg { return jobsLoadedMsg{jobs: nil} }
}

// dispatchDelete calls the right delete endpoint based on repo context.
func dispatchDelete(client *api.Client, m model) tea.Cmd {
	if m.repos.selectedOrg != nil && m.repos.selectedRepo != nil {
		return deleteJob(client, m.repos.selectedOrg.ID, m.repos.selectedRepo.ID, m.selectedJob.ID)
	}
	return deleteJobByID(client, m.selectedJob.ID)
}

// dispatchCreate routes job creation to the right API endpoint.
func dispatchCreate(client *api.Client, m model, steps []api.Step) tea.Msg {
	if m.repos.selectedGH != nil {
		cloneURL := preferredGitHubCloneURL(m.repos.selectedGH)
		if cloneURL == "" {
			return errMsg("no clone url found for selected GitHub repository")
		}
		if err := client.CreateJobWithURL(cloneURL, steps); err != nil {
			return errMsg(err.Error())
		}
		return jobCreatedMsg{}
	}
	if m.repos.selectedOrg != nil && m.repos.selectedRepo != nil {
		if err := client.CreateJob(m.repos.selectedOrg.ID, m.repos.selectedRepo.ID, steps); err != nil {
			return errMsg(err.Error())
		}
		return jobCreatedMsg{}
	}
	return errMsg("no repository context for job creation")
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func doLogin(client *api.Client, email, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Login(email, password)
		if err != nil {
			return errMsg(err.Error())
		}
		return loginSuccessMsg{token: resp.AccessToken}
	}
}

func waitForGitHubToken(baseURL string) tea.Cmd {
	return func() tea.Msg {
		ln, err := net.Listen("tcp", githubOAuthCallbackAddr)
		if err != nil {
			return errMsg(fmt.Sprintf("impossible de démarrer le callback GitHub sur %s: %v", githubOAuthCallbackAddr, err))
		}

		tokenChan := make(chan string, 1)
		callbackErrChan := make(chan error, 1)
		listenErrChan := make(chan error, 1)

		mux := http.NewServeMux()
		srv := &http.Server{Handler: mux}
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimSpace(r.URL.Query().Get("token"))
			if token == "" {
				http.Error(w, "missing token", http.StatusBadRequest)
				select {
				case callbackErrChan <- fmt.Errorf("token manquant dans le callback"):
				default:
				}
				return
			}

			_, _ = fmt.Fprintln(w, "Token received! You can return to the CLI.")
			select {
			case tokenChan <- token:
			default:
			}
		})

		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				listenErrChan <- err
			}
		}()

		if err := openGitHubLogin(baseURL); err != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return errMsg(err.Error())
		}

		timer := time.NewTimer(githubOAuthTimeout)
		defer timer.Stop()

		var token string
		select {
		case token = <-tokenChan:
		case err := <-callbackErrChan:
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return errMsg("callback GitHub invalide: " + err.Error())
		case err := <-listenErrChan:
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return errMsg("serveur callback GitHub indisponible: " + err.Error())
		case <-timer.C:
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = srv.Shutdown(shutdownCtx)
			cancel()
			return errMsg("connexion GitHub expirée: aucun token reçu")
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		cancel()

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

func githubAuthURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = config.DefaultAPIURL
	}
	return strings.TrimRight(trimmed, "/") + "/auth/github"
}

func openGitHubLogin(baseURL string) error {
	authURL := githubAuthURL(baseURL)

	var openers [][]string
	switch runtime.GOOS {
	case "darwin":
		openers = [][]string{{"open", authURL}}
	case "windows":
		openers = [][]string{{"rundll32", "url.dll,FileProtocolHandler", authURL}}
	default:
		openers = [][]string{
			{"xdg-open", authURL},
			{"gio", "open", authURL},
			{"sensible-browser", authURL},
		}
	}

	for _, opener := range openers {
		if len(opener) == 0 {
			continue
		}
		cmd := exec.Command(opener[0], opener[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("impossible d'ouvrir le navigateur automatiquement. Ouvrez cette URL: %s", authURL)
}

func startAuthTicker() tea.Cmd {
	return tea.Every(30*time.Second, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func startJobsStreamRetryTicker(delay time.Duration) tea.Cmd {
	if delay <= 0 {
		delay = time.Second
	}
	return tea.Tick(delay, func(_ time.Time) tea.Msg {
		return jobsStreamRetryTickMsg{}
	})
}

type jobsStreamFilter struct {
	orgID     string
	repoID    string
	cloneURLs []string
}

func openJobsStream(client *api.Client, m model) tea.Cmd {
	return func() tea.Msg {
		filter, ok := jobsStreamFilterFromModel(m)
		if !ok {
			return jobsStreamFailedMsg{err: "no repository context for jobs stream"}
		}

		ctx, cancel := context.WithCancel(context.Background())
		messages := make(chan tea.Msg, 8)
		go runJobsSSEStream(ctx, client, filter, messages)

		return jobsStreamOpenedMsg{messages: messages, stop: cancel}
	}
}

func waitNextJobsStreamMessage(messages <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-messages
		if !ok {
			return jobsStreamClosedMsg{}
		}
		return msg
	}
}

func jobsStreamFilterFromModel(m model) (jobsStreamFilter, bool) {
	if m.repos.selectedGH != nil {
		return jobsStreamFilter{cloneURLs: githubCloneURLCandidates(m.repos.selectedGH)}, true
	}
	if m.repos.selectedOrg != nil && m.repos.selectedRepo != nil {
		return jobsStreamFilter{orgID: m.repos.selectedOrg.ID, repoID: m.repos.selectedRepo.ID}, true
	}
	return jobsStreamFilter{}, false
}

func runJobsSSEStream(ctx context.Context, client *api.Client, filter jobsStreamFilter, messages chan<- tea.Msg) {
	defer close(messages)

	req, err := buildJobsStreamRequest(ctx, client.BaseURL, filter)
	if err != nil {
		sendJobsStreamMessage(ctx, messages, jobsStreamFailedMsg{err: "invalid jobs stream request"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+client.Token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	streamHTTP := &http.Client{Transport: client.HTTPClient().Transport}
	resp, err := streamHTTP.Do(req)
	if err != nil {
		sendJobsStreamMessage(ctx, messages, jobsStreamFailedMsg{err: "jobs stream unavailable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		sendJobsStreamMessage(ctx, messages, jobsStreamFailedMsg{err: "401 unauthorized"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		summary := strings.TrimSpace(string(body))
		if summary == "" {
			summary = http.StatusText(resp.StatusCode)
		}
		sendJobsStreamMessage(ctx, messages, jobsStreamFailedMsg{err: fmt.Sprintf("jobs stream failed: %d (%s)", resp.StatusCode, summary)})
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var eventName string
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			if len(dataLines) == 0 {
				eventName = ""
				continue
			}

			data := strings.Join(dataLines, "\n")
			dataLines = nil

			if eventName == "ping" {
				eventName = ""
				continue
			}

			if eventMsg, ok := parseJobsStreamPayload(data); ok {
				sendJobsStreamMessage(ctx, messages, eventMsg)
			}
			eventName = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		sendJobsStreamMessage(ctx, messages, jobsStreamFailedMsg{err: "jobs stream disconnected"})
	}
}

func sendJobsStreamMessage(ctx context.Context, messages chan<- tea.Msg, msg tea.Msg) {
	select {
	case <-ctx.Done():
		return
	case messages <- msg:
		return
	}
}

func buildJobsStreamRequest(ctx context.Context, baseURL string, filter jobsStreamFilter) (*http.Request, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/jobs/stream")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if filter.orgID != "" {
		q.Set("org_id", filter.orgID)
	}
	if filter.repoID != "" {
		q.Set("repo_id", filter.repoID)
	}
	for _, cloneURL := range filter.cloneURLs {
		if trimmed := strings.TrimSpace(cloneURL); trimmed != "" {
			q.Add("clone_url", trimmed)
		}
	}
	u.RawQuery = q.Encode()

	return http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
}

func parseJobsStreamPayload(data string) (jobsStreamEventMsg, bool) {
	var allJobs []api.Job
	if err := json.Unmarshal([]byte(data), &allJobs); err == nil {
		return jobsStreamEventMsg{jobs: allJobs, full: true}, true
	}

	var payload struct {
		Jobs []api.Job `json:"jobs"`
		Job  *api.Job  `json:"job"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err == nil {
		if payload.Jobs != nil {
			return jobsStreamEventMsg{jobs: payload.Jobs, full: true}, true
		}
		if payload.Job != nil {
			return jobsStreamEventMsg{jobs: []api.Job{*payload.Job}, full: false}, true
		}
	}

	var single api.Job
	if err := json.Unmarshal([]byte(data), &single); err == nil && single.ID != "" {
		return jobsStreamEventMsg{jobs: []api.Job{single}, full: false}, true
	}

	return jobsStreamEventMsg{}, false
}

// checkAuthCmd pings the API. Only propagates 401 errors to avoid spurious logouts on network hiccups.
func checkAuthCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := client.CheckAuth(); err != nil {
			if strings.Contains(err.Error(), "401") {
				return errMsg(err.Error())
			}
			// Network error — ignore silently to avoid forced logout on timeout.
			return nil
		}
		return nil
	}
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

func fetchJobs(client *api.Client, orgID, repoID string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetJobs(orgID, repoID)
		if err != nil {
			return errMsg(err.Error())
		}
		return jobsLoadedMsg{jobs: jobs}
	}
}

func fetchAllJobs(client *api.Client, cloneURLs ...string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetAllJobs()
		if err != nil {
			return errMsg(err.Error())
		}

		targets := make(map[string]struct{})
		for _, raw := range cloneURLs {
			url := strings.TrimSpace(raw)
			if url != "" {
				targets[url] = struct{}{}
			}
		}

		if len(targets) == 0 {
			return jobsLoadedMsg{jobs: nil}
		}

		var filtered []api.Job
		for _, j := range jobs {
			if _, ok := targets[j.CloneURL]; ok {
				filtered = append(filtered, j)
			}
		}
		return jobsLoadedMsg{jobs: filtered}
	}
}

func preferredGitHubCloneURL(repo *api.GitHubRepo) string {
	if repo == nil {
		return ""
	}
	if ssh := strings.TrimSpace(repo.SSHURL); ssh != "" {
		return ssh
	}
	return strings.TrimSpace(repo.CloneURL)
}

func githubCloneURLCandidates(repo *api.GitHubRepo) []string {
	if repo == nil {
		return nil
	}
	urls := make([]string, 0, 2)
	if ssh := strings.TrimSpace(repo.SSHURL); ssh != "" {
		urls = append(urls, ssh)
	}
	if https := strings.TrimSpace(repo.CloneURL); https != "" && !containsString(urls, https) {
		urls = append(urls, https)
	}
	return urls
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func deleteJob(client *api.Client, orgID, repoID, jobID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteJob(orgID, repoID, jobID); err != nil {
			return errMsg(err.Error())
		}
		return jobDeletedMsg{}
	}
}

func deleteJobByID(client *api.Client, jobID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteJobByID(jobID); err != nil {
			return errMsg(err.Error())
		}
		return jobDeletedMsg{}
	}
}

func downloadJobArtifact(client *api.Client, jobID string, repoName string) tea.Cmd {
	return func() tea.Msg {
		path, err := saveJobArtifactZip(client, jobID, repoName)
		if err != nil {
			return errMsg(err.Error())
		}
		return artifactDownloadedMsg{path: path}
	}
}

func saveJobArtifactZip(client *api.Client, jobID string, repoName string) (string, error) {
	content, err := client.DownloadJobArtifact(jobID)
	if err != nil {
		return "", err
	}

	name := artifactZipFileName(jobID, repoName, time.Now())
	for _, candidate := range artifactWriteCandidates(name) {
		if err := writeArtifactFile(candidate, content); err != nil {
			continue
		}

		absPath, err := filepath.Abs(candidate)
		if err != nil {
			return candidate, nil
		}
		return absPath, nil
	}

	return "", fmt.Errorf("artifact save failed")
}

// artifactWriteCandidates returns where an artifact may be saved, in order of
// preference: ~/Downloads/gopherci, then ~/gopherci-artifacts. Both live in the
// user's home directory so other local users cannot read or hijack them.
func artifactWriteCandidates(name string) []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Downloads", "gopherci", name),
		filepath.Join(home, "gopherci-artifacts", name),
	}
}

// writeArtifactFile creates a new private file; it never follows or overwrites
// an existing file.
func writeArtifactFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

func artifactZipFileName(jobID string, repoName string, now time.Time) string {
	baseID := sanitizeFileSegment(shortID(strings.TrimSpace(jobID)))
	if baseID == "" {
		baseID = "job"
	}
	repoSegment := ""
	if trimmedRepo := strings.TrimSpace(repoName); trimmedRepo != "" {
		repoSegment = sanitizeFileSegment(trimmedRepo)
	}
	if repoSegment != "" {
		return fmt.Sprintf("%s-%s-%s.zip", repoSegment, baseID, now.Format("20060102-150405"))
	}
	return fmt.Sprintf("gopherci-artifact-%s-%s.zip", baseID, now.Format("20060102-150405"))
}

func sanitizeFileSegment(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "job"
	}
	return out
}

func createJobFromGoci(client *api.Client, m model) tea.Cmd {
	return func() tea.Msg {
		if len(m.newJob.steps) == 0 {
			return errMsg("no steps defined in .goci")
		}
		return dispatchCreate(client, m, m.newJob.steps)
	}
}

func createJobFromCmd(client *api.Client, m model, cmd string) tea.Cmd {
	return func() tea.Msg {
		steps := []api.Step{
			{Name: "run", Cmd: []string{"sh", "-c", cmd}},
		}
		return dispatchCreate(client, m, steps)
	}
}

func rerunJob(client *api.Client, orgID, repoID string) tea.Cmd {
	return func() tea.Msg {
		steps, _, err := loadSteps()
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
		steps, _, err := loadSteps()
		if err != nil {
			return errMsg(err.Error())
		}
		if err = client.CreateJobWithURL(cloneURL, steps); err != nil {
			return errMsg(err.Error())
		}
		return jobCreatedMsg{}
	}
}

// ── Orgs / Repos ──────────────────────────────────────────────────────────────

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

// ── SSH Keys ──────────────────────────────────────────────────────────────────

func fetchSSHKeys(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		keys, err := client.GetSSHKeys()
		if err != nil {
			return errMsg(err.Error())
		}
		return sshKeysLoadedMsg{keys: keys}
	}
}

func createSSHKey(client *api.Client, name, privateKey string) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.CreateSSHKey(name, privateKey); err != nil {
			return errMsg(err.Error())
		}
		return sshKeyCreatedMsg{}
	}
}

func deleteSSHKey(client *api.Client, keyID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteSSHKey(keyID); err != nil {
			return errMsg(err.Error())
		}
		return sshKeyDeletedMsg{}
	}
}

// ── Clipboard ────────────────────────────────────────────────────────────────

func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return errMsg("clipboard: " + err.Error())
		}
		return clipboardCopiedMsg{}
	}
}
