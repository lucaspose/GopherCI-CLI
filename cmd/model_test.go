package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

// newTestModel builds a minimal model ready for unit testing.
func newTestModel() model {
	email := textinput.New()
	pass := textinput.New()
	return model{
		screen:    screenMenu,
		config:    &config.Config{APIURL: "http://test", Token: "tok"},
		apiClient: api.NewClient("http://test", "tok"),
		login: loginState{
			inputs: []textinput.Model{email, pass},
		},
		jobs: jobsState{pageSize: 15},
	}
}

func updateModel(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

// ── Message handlers ──────────────────────────────────────────────────────────

func TestUpdate_LoginSuccess_TransitionsToMenu(t *testing.T) {
	m := newTestModel()
	m.screen = screenLogin
	m = updateModel(m, loginSuccessMsg{token: "newtoken"})
	if m.screen != screenMenu {
		t.Errorf("expected screenMenu, got %d", m.screen)
	}
	if m.config.Token != "newtoken" {
		t.Errorf("expected token newtoken, got %s", m.config.Token)
	}
}

func TestUpdate_ErrMsg_SetsError(t *testing.T) {
	m := newTestModel()
	m = updateModel(m, errMsg("something went wrong"))
	if m.err != "Une action n'a pas pu être terminée. Réessayez." {
		t.Errorf("unexpected err: %s", m.err)
	}
}

func TestUpdate_ErrMsg_JobsServerError_IsSanitized(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m = updateModel(m, errMsg("get jobs failed: 500"))

	if m.err != "Le service est temporairement indisponible. Réessayez dans un instant." {
		t.Fatalf("unexpected sanitized error: %q", m.err)
	}
	if strings.Contains(m.err, "500") {
		t.Fatalf("error should not expose HTTP code, got %q", m.err)
	}
}

func TestUpdate_ErrMsg_ArtifactNotFound_IsSanitized(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m = updateModel(m, errMsg("download artifact failed: 404"))

	if m.err != "Aucun artifact ZIP disponible pour ce job." {
		t.Fatalf("unexpected artifact message: %q", m.err)
	}
}

func TestUpdate_ErrMsg_ArtifactSaveFailed_IsSanitized(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m = updateModel(m, errMsg("artifact save failed"))

	if m.err != "ZIP reçu, mais impossible de l'enregistrer localement." {
		t.Fatalf("unexpected save message: %q", m.err)
	}
}

func TestUpdate_Login401_ShowsFriendlyMessage(t *testing.T) {
	m := newTestModel()
	m.screen = screenLogin
	m = updateModel(m, errMsg("login failed: 401"))

	if m.screen != screenLogin {
		t.Fatalf("expected to stay on login, got %d", m.screen)
	}
	if m.err != "Connexion refusée. Vérifiez vos identifiants." {
		t.Fatalf("unexpected login error message: %q", m.err)
	}
}

func TestUpdate_401_RedirectsToLogin(t *testing.T) {
	m := newTestModel()
	m = updateModel(m, errMsg("401 unauthorized"))
	if m.screen != screenLogin {
		t.Errorf("expected screenLogin on 401, got %d", m.screen)
	}
	if m.config.Token != "" {
		t.Errorf("expected token cleared, got %s", m.config.Token)
	}
	if m.err != "" {
		t.Errorf("expected err cleared after 401, got %s", m.err)
	}
}

func TestUpdate_JobsLoaded_ClearsLoading(t *testing.T) {
	m := newTestModel()
	m.loading = true
	jobs := []api.Job{{ID: "job1", Status: "success"}}
	m = updateModel(m, jobsLoadedMsg{jobs: jobs})
	if m.loading {
		t.Error("expected loading to be false")
	}
	if len(m.jobs.jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(m.jobs.jobs))
	}
	if m.jobs.cursor != 0 {
		t.Errorf("expected cursor reset to 0")
	}
}

func TestUpdate_JobsStreamEvent_PreservesCursorAndPage(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.jobs = []api.Job{
		{ID: "job1", Status: "success"},
		{ID: "job2", Status: "running"},
		{ID: "job3", Status: "failed"},
	}
	m.jobs.cursor = 2
	m.jobs.pageOffset = 0
	m.jobsStream.messages = make(chan tea.Msg)

	jobs := []api.Job{
		{ID: "job1", Status: "success"},
		{ID: "job2", Status: "running"},
		{ID: "job3", Status: "failed"},
	}
	m = updateModel(m, jobsStreamEventMsg{jobs: jobs, full: true})

	if m.jobs.cursor != 2 {
		t.Fatalf("expected cursor to be preserved, got %d", m.jobs.cursor)
	}
	if m.jobs.pageOffset != 0 {
		t.Fatalf("expected page offset to be preserved, got %d", m.jobs.pageOffset)
	}
}

func TestUpdate_JobsStreamEvent_UpdatesSelectedJob(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m.jobsStream.messages = make(chan tea.Msg)
	m.jobs.jobs = []api.Job{{ID: "job1", Status: "running"}}
	m.selectedJob = &m.jobs.jobs[0]

	m = updateModel(m, jobsStreamEventMsg{jobs: []api.Job{{ID: "job1", Status: "success"}}, full: true})

	if m.selectedJob == nil {
		t.Fatal("expected selected job to stay set")
	}
	if m.selectedJob.Status != "success" {
		t.Fatalf("expected selected job status to be updated to success, got %q", m.selectedJob.Status)
	}
}

func TestUpdate_JobsStreamEvent_MergesPartialUpdate(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.jobs = []api.Job{{ID: "job1", Status: "running"}}
	m.jobs.cursor = 0
	m.jobs.pageOffset = 0
	m.jobsStream.messages = make(chan tea.Msg)

	m = updateModel(m, jobsStreamEventMsg{jobs: []api.Job{{ID: "job1", Status: "success"}}, full: false})

	if len(m.jobs.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(m.jobs.jobs))
	}
	if m.jobs.jobs[0].Status != "success" {
		t.Fatalf("expected job status to be updated, got %q", m.jobs.jobs[0].Status)
	}
}

func TestUpdate_JobsStreamFailed401_RedirectsToLogin(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m = updateModel(m, jobsStreamFailedMsg{err: "401 unauthorized"})

	if m.screen != screenLogin {
		t.Fatalf("expected screenLogin on stream 401, got %d", m.screen)
	}
	if m.config.Token != "" {
		t.Fatalf("expected token cleared on stream 401, got %q", m.config.Token)
	}
}

func TestUpdate_JobDeleted_TransitionsToJobs(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m.actions.confirmDelete = true
	m = updateModel(m, jobDeletedMsg{})
	if m.screen != screenJobs {
		t.Errorf("expected screenJobs after delete, got %d", m.screen)
	}
	if m.actions.confirmDelete {
		t.Error("expected confirmDelete to be cleared")
	}
}

func TestUpdate_JobCreated_TransitionsToJobs(t *testing.T) {
	m := newTestModel()
	m.screen = screenNewJob
	m = updateModel(m, jobCreatedMsg{})
	if m.screen != screenJobs {
		t.Errorf("expected screenJobs after create, got %d", m.screen)
	}
}

func TestHandleActionEnter_DownloadArtifact(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	job := api.Job{ID: "job-123"}
	m.selectedJob = &job
	m.actions.cursor = 2

	next, cmd := m.handleActionEnter()
	nm := next.(model)

	if !nm.loading {
		t.Fatal("expected loading=true when downloading artifact")
	}
	if nm.loadingMsg != "Téléchargement de l'artifact..." {
		t.Fatalf("unexpected loading message: %q", nm.loadingMsg)
	}
	if cmd == nil {
		t.Fatal("expected download command to be returned")
	}
}

func TestUpdate_SSHKeysLoaded(t *testing.T) {
	m := newTestModel()
	m.loading = true
	keys := []api.SSHKey{{ID: "k1", Name: "deploy"}}
	m = updateModel(m, sshKeysLoadedMsg{keys: keys})
	if m.loading {
		t.Error("expected loading false")
	}
	if len(m.ssh.keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(m.ssh.keys))
	}
}

func TestUpdate_SSHKeyDeleted_ClearsConfirm(t *testing.T) {
	m := newTestModel()
	m.ssh.confirmDelete = true
	m = updateModel(m, sshKeyDeletedMsg{})
	if m.ssh.confirmDelete {
		t.Error("expected confirmDelete false after delete")
	}
}

func TestUpdate_SSHKeyCreated_ResetsMode(t *testing.T) {
	m := newTestModel()
	m.ssh.mode = 2
	m = updateModel(m, sshKeyCreatedMsg{})
	if m.ssh.mode != 0 {
		t.Errorf("expected mode 0, got %d", m.ssh.mode)
	}
}

func TestUpdate_ClipboardCopied_SetsSuccessMsg(t *testing.T) {
	m := newTestModel()
	m = updateModel(m, clipboardCopiedMsg{})
	if m.successMsg == "" {
		t.Error("expected successMsg to be set")
	}
}

func TestUpdate_OrgsLoaded(t *testing.T) {
	m := newTestModel()
	m.loading = true
	orgs := []api.Organization{{ID: "o1", Name: "my-org"}}
	m = updateModel(m, orgsLoadedMsg{orgs: orgs})
	if m.loading {
		t.Error("expected loading false")
	}
	if len(m.repos.orgs) != 1 {
		t.Errorf("expected 1 org, got %d", len(m.repos.orgs))
	}
	if m.repos.subMode != 0 {
		t.Errorf("expected subMode 0, got %d", m.repos.subMode)
	}
}

func TestUpdate_ReposLoaded(t *testing.T) {
	m := newTestModel()
	m.loading = true
	repos := []api.Repository{{ID: "r1", Name: "backend"}}
	m = updateModel(m, reposLoadedMsg{repos: repos})
	if m.loading {
		t.Error("expected loading false")
	}
	if len(m.repos.repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(m.repos.repos))
	}
	if m.repos.subMode != 1 {
		t.Errorf("expected subMode 1, got %d", m.repos.subMode)
	}
}

// ── Key handlers ──────────────────────────────────────────────────────────────

func handleKeyStr(m model, key string) model {
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return next.(model)
}

func handleSpecialKey(m model, key tea.KeyType) model {
	next, _ := m.handleKey(tea.KeyMsg{Type: key})
	return next.(model)
}

func TestHandleKey_EscFromLogs_GoesToActions(t *testing.T) {
	m := newTestModel()
	m.screen = screenLogs
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.screen != screenJobsActions {
		t.Errorf("expected screenJobsActions, got %d", m.screen)
	}
}

func TestHandleKey_EscFromActions_GoesToJobs(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.screen != screenJobs {
		t.Errorf("expected screenJobs, got %d", m.screen)
	}
}

func TestHandleKey_EscFromJobs_GoesToMenu(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.screen != screenMenu {
		t.Errorf("expected screenMenu, got %d", m.screen)
	}
}

func TestHandleKey_EscFromHelp_GoesToMenu(t *testing.T) {
	m := newTestModel()
	m.screen = screenHelp
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.screen != screenMenu {
		t.Errorf("expected screenMenu, got %d", m.screen)
	}
}

func TestHandleKey_EscFromSettings_GoesToMenu(t *testing.T) {
	m := newTestModel()
	m.screen = screenSettings
	m.settings.editing = false
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.screen != screenMenu {
		t.Errorf("expected screenMenu, got %d", m.screen)
	}
}

func TestHandleKey_EscCancelsSettingsEditing(t *testing.T) {
	m := newTestModel()
	m.screen = screenSettings
	m.settings.editing = true
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.settings.editing {
		t.Error("expected editing to be cancelled")
	}
	if m.screen != screenSettings {
		t.Errorf("expected to stay on settings, got %d", m.screen)
	}
}

func TestHandleKey_D_SetsConfirmDelete_SSH(t *testing.T) {
	m := newTestModel()
	m.screen = screenSSHKeys
	m.ssh.mode = 0
	m.ssh.keys = []api.SSHKey{{ID: "k1", Name: "deploy"}}
	m.ssh.cursor = 0
	m = handleKeyStr(m, "d")
	if !m.ssh.confirmDelete {
		t.Error("expected confirmDelete true after d press")
	}
}

func TestHandleKey_D_DoesNotImmediatelyDelete(t *testing.T) {
	m := newTestModel()
	m.screen = screenSSHKeys
	m.ssh.mode = 0
	m.ssh.keys = []api.SSHKey{{ID: "k1", Name: "deploy"}}
	before := len(m.ssh.keys)
	m = handleKeyStr(m, "d")
	if len(m.ssh.keys) != before {
		t.Error("keys should not be deleted immediately on d press")
	}
}

func TestHandleKey_EscCancelsConfirmDelete_SSH(t *testing.T) {
	m := newTestModel()
	m.screen = screenSSHKeys
	m.ssh.mode = 0
	m.ssh.confirmDelete = true
	m.ssh.keys = []api.SSHKey{{ID: "k1"}}
	m = handleSpecialKey(m, tea.KeyEsc)
	if m.ssh.confirmDelete {
		t.Error("expected confirmDelete false after esc")
	}
	if m.screen != screenSSHKeys {
		t.Errorf("expected to stay on sshkeys, got %d", m.screen)
	}
}

func TestHandleKey_F_CyclesFilter(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.filter = ""
	m = handleKeyStr(m, "f")
	if m.jobs.filter == "" {
		t.Error("filter should change from empty after f press")
	}
	// Cycle through remaining 4 statuses back to empty (5 total = full cycle)
	for i := 0; i < 4; i++ {
		m = handleKeyStr(m, "f")
	}
	if m.jobs.filter != "" {
		t.Errorf("expected filter back to empty after full cycle, got %q", m.jobs.filter)
	}
}

func TestHandleKey_F_ResetsPageOffset(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.pageOffset = 30
	m = handleKeyStr(m, "f")
	if m.jobs.pageOffset != 0 {
		t.Errorf("expected pageOffset reset to 0, got %d", m.jobs.pageOffset)
	}
}

func TestHandleKey_SuccessMsgClearedOnKeypress(t *testing.T) {
	m := newTestModel()
	m.successMsg = "some message"
	m = handleKeyStr(m, "f")
	if m.successMsg != "" {
		t.Errorf("expected successMsg cleared on keypress, got %q", m.successMsg)
	}
}

func TestUpdate_LoginInput_AllowsLetters(t *testing.T) {
	m := newTestModel()
	m.screen = screenLogin
	m.login.focused = 0
	m.login.inputs[0].Focus()

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if got := m.login.inputs[0].Value(); got != "f" {
		t.Fatalf("expected login input to contain 'f', got %q", got)
	}
}

func TestUpdate_JobsShortcutStillHandled(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.filter = ""

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if m.jobs.filter == "" {
		t.Fatal("expected filter shortcut to be handled on jobs screen")
	}
}

func TestNormalizeSSHPrivateKey_RejectsPublicKey(t *testing.T) {
	if _, err := normalizeSSHPrivateKey("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@host"); err == nil {
		t.Fatal("expected error when a public key is provided")
	}
}

func TestNormalizeSSHPrivateKey_AcceptsRawPrivateKey(t *testing.T) {
	raw := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	got, err := normalizeSSHPrivateKey(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != raw {
		t.Fatalf("unexpected normalized value: got %q want %q", got, raw)
	}
}

func TestNormalizeSSHPrivateKey_ReadsFromPath(t *testing.T) {
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	filePath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(filePath, []byte(privateKey), 0600); err != nil {
		t.Fatalf("write private key file: %v", err)
	}

	got, err := normalizeSSHPrivateKey(filePath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != privateKey {
		t.Fatalf("unexpected value read from file: got %q want %q", got, privateKey)
	}
}

// ── moveCursor ────────────────────────────────────────────────────────────────

func TestMoveCursor_Menu_Clamps(t *testing.T) {
	m := newTestModel()
	m.screen = screenMenu
	m.menuCursor = 0
	m.moveCursor(-1)
	if m.menuCursor != 0 {
		t.Errorf("expected 0 at min, got %d", m.menuCursor)
	}
	m.menuCursor = 5
	m.moveCursor(1)
	if m.menuCursor != 5 {
		t.Errorf("expected 5 at max, got %d", m.menuCursor)
	}
}

func TestMoveCursor_SSHKeys_Clamps(t *testing.T) {
	m := newTestModel()
	m.screen = screenSSHKeys
	m.ssh.mode = 0
	m.ssh.keys = []api.SSHKey{{ID: "k1"}, {ID: "k2"}}
	m.ssh.cursor = 0
	m.moveCursor(-1)
	if m.ssh.cursor != 0 {
		t.Errorf("expected 0 at min, got %d", m.ssh.cursor)
	}
	m.ssh.cursor = 1
	m.moveCursor(1)
	if m.ssh.cursor != 1 {
		t.Errorf("expected 1 at max, got %d", m.ssh.cursor)
	}
}

func TestMoveCursor_JobsActions_Clamps(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobsActions
	m.actions.cursor = 0
	m.moveCursor(-1)
	if m.actions.cursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", m.actions.cursor)
	}
	m.actions.cursor = 3
	m.moveCursor(1)
	if m.actions.cursor != 3 {
		t.Fatalf("expected cursor to stay at 3, got %d", m.actions.cursor)
	}
}

func TestHandleKey_RightLeft_PaginatesGitHubRepos(t *testing.T) {
	m := newTestModel()
	m.screen = screenRepos
	m.height = 16 // GitHub row budget = 3
	m.repos.githubRepos = []api.GitHubRepo{
		{FullName: "acme/repo-1", Name: "repo-1"},
		{FullName: "acme/repo-2", Name: "repo-2"},
		{FullName: "acme/repo-3", Name: "repo-3"},
		{FullName: "acme/repo-4", Name: "repo-4"},
		{FullName: "acme/repo-5", Name: "repo-5"},
		{FullName: "acme/repo-6", Name: "repo-6"},
		{FullName: "acme/repo-7", Name: "repo-7"},
		{FullName: "acme/repo-8", Name: "repo-8"},
		{FullName: "acme/repo-9", Name: "repo-9"},
	}

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.repos.githubPageOffset != 2 {
		t.Fatalf("expected github page offset 2 after right, got %d", m.repos.githubPageOffset)
	}
	if m.repos.githubCursor != 2 {
		t.Fatalf("expected github cursor 2 after right, got %d", m.repos.githubCursor)
	}

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.repos.githubPageOffset != 0 {
		t.Fatalf("expected github page offset 0 after left, got %d", m.repos.githubPageOffset)
	}
	if m.repos.githubCursor != 0 {
		t.Fatalf("expected github cursor 0 after left, got %d", m.repos.githubCursor)
	}
}

func TestGitHubRepoPageBounds_RespectsOwnerHeaderRows(t *testing.T) {
	repos := []api.GitHubRepo{
		{FullName: "cpandreau/ChambreDEcho", Name: "ChambreDEcho"},
		{FullName: "flavien87/Application-Rencontre", Name: "Application-Rencontre"},
		{FullName: "lucaspose/GopherCI", Name: "GopherCI"},
		{FullName: "lucaspose/GopherCI-CLI", Name: "GopherCI-CLI"},
	}

	start, end, totalPages, _, _ := githubRepoPageBounds(repos, 5, 0)
	if start != 0 {
		t.Fatalf("expected first page to start at 0, got %d", start)
	}
	if end != 2 {
		t.Fatalf("expected first page to stop before third owner, got end=%d", end)
	}
	if totalPages <= 1 {
		t.Fatalf("expected multiple pages, got %d", totalPages)
	}

	rows := 0
	ownerOnPage := ""
	for _, repo := range repos[start:end] {
		owner := githubRepoOwner(repo.FullName)
		if owner != ownerOnPage {
			rows++
			ownerOnPage = owner
		}
		rows++
	}
	if rows > 5 {
		t.Fatalf("expected at most 5 rendered rows, got %d", rows)
	}
}

func TestViewRepos_GitHub_FitsTerminalHeight(t *testing.T) {
	m := newTestModel()
	m.screen = screenRepos
	m.width = 120
	m.height = 20
	m.repos.githubRepos = []api.GitHubRepo{
		{FullName: "cpandreau/ChambreDEcho", Name: "ChambreDEcho", Private: true},
		{FullName: "flavien87/Application-Rencontre", Name: "Application-Rencontre", Private: true},
		{FullName: "lucaspose/CI-CD_Discord", Name: "CI-CD_Discord", Private: true},
		{FullName: "lucaspose/digitalys", Name: "digitalys", Private: true},
		{FullName: "lucaspose/GopherCI", Name: "GopherCI", Private: true},
		{FullName: "lucaspose/GopherCI-CLI", Name: "GopherCI-CLI", Private: true},
		{FullName: "lucaspose/Portfolio", Name: "Portfolio", Private: true},
		{FullName: "lucaspose/Risky-Gamble", Name: "Risky-Gamble", Private: true},
		{FullName: "lucaspose/SchoolPulse", Name: "SchoolPulse", Private: false},
		{FullName: "lucaspose/Terminalis", Name: "Terminalis", Private: true},
	}

	out := viewRepos(m)
	trimmed := strings.TrimSuffix(out, "\n")
	lineCount := 0
	if trimmed != "" {
		lineCount = strings.Count(trimmed, "\n") + 1
	}

	if lineCount > m.height {
		t.Fatalf("expected repos view to fit in terminal height %d, got %d lines", m.height, lineCount)
	}
}

func TestMoveCursor_Repos_StaysWithinCurrentPage(t *testing.T) {
	m := newTestModel()
	m.screen = screenRepos
	m.height = 16 // repos page size = 5
	m.repos.subMode = 1
	m.repos.repos = []api.Repository{
		{ID: "r1", Name: "repo-1"},
		{ID: "r2", Name: "repo-2"},
		{ID: "r3", Name: "repo-3"},
		{ID: "r4", Name: "repo-4"},
		{ID: "r5", Name: "repo-5"},
		{ID: "r6", Name: "repo-6"},
	}

	for i := 0; i < 10; i++ {
		m.moveCursor(+1)
	}

	if m.repos.repoCursor != 4 {
		t.Fatalf("expected cursor to stop at page end (4), got %d", m.repos.repoCursor)
	}
	if m.repos.repoPageOffset != 0 {
		t.Fatalf("expected page offset to stay at 0 with only down key, got %d", m.repos.repoPageOffset)
	}
}

func TestUpdate_WindowResize_RecomputesJobsPageSize(t *testing.T) {
	m := newTestModel()
	m.screen = screenJobs
	m.jobs.jobs = make([]api.Job, 20)
	for i := range m.jobs.jobs {
		m.jobs.jobs[i] = api.Job{ID: "job" + string(rune('a'+i))}
	}
	m.jobs.cursor = 14
	m.jobs.pageOffset = 0

	m = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 18})

	if m.jobs.pageSize != 6 {
		t.Fatalf("expected jobs page size 6 after resize, got %d", m.jobs.pageSize)
	}
	if m.jobs.pageOffset != 12 {
		t.Fatalf("expected jobs page offset 12 after resize, got %d", m.jobs.pageOffset)
	}
	if m.jobs.cursor != 14 {
		t.Fatalf("expected jobs cursor to stay on selected row, got %d", m.jobs.cursor)
	}
}
