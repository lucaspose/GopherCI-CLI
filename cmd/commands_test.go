package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

func TestGitHubAuthURL_WithTrailingSlash(t *testing.T) {
	got := githubAuthURL(config.DefaultAPIURL + "/")
	if got != strings.TrimRight(config.DefaultAPIURL, "/")+"/auth/github" {
		t.Fatalf("unexpected auth url: %s", got)
	}
}

func TestGitHubAuthURL_WithEmpty(t *testing.T) {
	got := githubAuthURL("   ")
	if got != strings.TrimRight(config.DefaultAPIURL, "/")+"/auth/github" {
		t.Fatalf("unexpected auth url for empty base: %s", got)
	}
}

func TestPreferredGitHubCloneURL_PrefersSSH(t *testing.T) {
	repo := &api.GitHubRepo{
		CloneURL: "https://github.com/example/repo.git",
		SSHURL:   "git@github.com:example/repo.git",
	}

	got := preferredGitHubCloneURL(repo)
	if got != repo.SSHURL {
		t.Fatalf("expected SSH URL %q, got %q", repo.SSHURL, got)
	}
}

func TestPreferredGitHubCloneURL_FallbackToHTTPS(t *testing.T) {
	repo := &api.GitHubRepo{CloneURL: "https://github.com/example/repo.git"}

	got := preferredGitHubCloneURL(repo)
	if got != repo.CloneURL {
		t.Fatalf("expected HTTPS URL %q, got %q", repo.CloneURL, got)
	}
}

func TestGitHubCloneURLCandidates_IncludesSSHAndHTTPS(t *testing.T) {
	repo := &api.GitHubRepo{
		CloneURL: "https://github.com/example/repo.git",
		SSHURL:   "git@github.com:example/repo.git",
	}

	got := githubCloneURLCandidates(repo)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d (%v)", len(got), got)
	}
	if got[0] != repo.SSHURL {
		t.Fatalf("expected first candidate to be SSH URL, got %q", got[0])
	}
	if got[1] != repo.CloneURL {
		t.Fatalf("expected second candidate to be HTTPS URL, got %q", got[1])
	}
}

func TestParseJobsStreamPayload_FullList(t *testing.T) {
	msg, ok := parseJobsStreamPayload(`[{"id":"job1","status":"running"}]`)
	if !ok {
		t.Fatal("expected payload to be parsed")
	}
	if !msg.full {
		t.Fatal("expected full snapshot payload")
	}
	if len(msg.jobs) != 1 || msg.jobs[0].ID != "job1" {
		t.Fatalf("unexpected parsed jobs: %+v", msg.jobs)
	}
}

func TestParseJobsStreamPayload_SingleJob(t *testing.T) {
	msg, ok := parseJobsStreamPayload(`{"job":{"id":"job2","status":"success"}}`)
	if !ok {
		t.Fatal("expected payload to be parsed")
	}
	if msg.full {
		t.Fatal("expected partial update payload")
	}
	if len(msg.jobs) != 1 || msg.jobs[0].ID != "job2" {
		t.Fatalf("unexpected parsed jobs: %+v", msg.jobs)
	}
}

func TestBuildJobsStreamRequest_EncodesFilter(t *testing.T) {
	req, err := buildJobsStreamRequest(context.Background(), "http://localhost:8080", jobsStreamFilter{
		orgID:     "org1",
		repoID:    "repo1",
		cloneURLs: []string{"git@github.com:acme/repo.git", "https://github.com/acme/repo.git"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(req.URL.String(), "http://localhost:8080/jobs/stream") {
		t.Fatalf("unexpected stream url: %s", req.URL.String())
	}
	q := req.URL.Query()
	if q.Get("org_id") != "org1" || q.Get("repo_id") != "repo1" {
		t.Fatalf("unexpected org/repo query values: %v", q)
	}
	if got := q["clone_url"]; len(got) != 2 {
		t.Fatalf("expected two clone_url values, got %v", got)
	}
}

func TestArtifactZipFileName(t *testing.T) {
	now := time.Date(2026, 4, 21, 10, 30, 45, 0, time.UTC)
	got := artifactZipFileName("abc12345-deadbeef", "", now)
	want := "gopherci-artifact-abc12345-20260421-103045.zip"
	if got != want {
		t.Fatalf("unexpected artifact filename: got %q want %q", got, want)
	}
}

func TestSaveJobArtifactZip(t *testing.T) {
	artifact := []byte("fake-zip-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/jobs/job1/artifact" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer tok" {
			t.Fatalf("unexpected auth header: %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifact)
	}))
	defer server.Close()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	client := api.NewClient(server.URL, "tok")
	path, err := saveJobArtifactZip(client, "job1", "")
	if err != nil {
		t.Fatalf("saveJobArtifactZip: %v", err)
	}
	if filepath.Ext(path) != ".zip" {
		t.Fatalf("expected .zip artifact file, got %q", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved artifact: %v", err)
	}
	if string(content) != string(artifact) {
		t.Fatalf("unexpected saved content: %q", string(content))
	}
}
