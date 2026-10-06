//go:build integration

// Integration tests against a running GopherCI server:
//
//	GOCI_API_URL=http://localhost:8080 go test -tags integration ./internal/api/
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAgainstServer(t *testing.T) {
	base := os.Getenv("GOCI_API_URL")
	if base == "" {
		t.Skip("GOCI_API_URL not set")
	}
	email := fmt.Sprintf("it-%d@example.com", time.Now().UnixNano())
	http.Post(base+"/users", "application/json", bytes.NewBufferString(`{"email":"`+email+`","password":"password123"}`))
	time.Sleep(1200 * time.Millisecond) // auth endpoints are rate limited

	login, err := NewClient(base, "").Login(email, "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	c := NewClient(base, login.AccessToken)
	must := func(step string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	post := func(path string, body any) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+login.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		must("POST "+path, err)
		resp.Body.Close()
	}
	waitJobs := func() []Job {
		t.Helper()
		for i := 0; i < 60; i++ {
			time.Sleep(time.Second)
			jobs, err := c.GetAllJobs()
			must("GetAllJobs", err)
			done := len(jobs) > 0
			for _, j := range jobs {
				if j.Status == "pending" || j.Status == "running" {
					done = false
				}
			}
			if done {
				return jobs
			}
		}
		t.Fatal("jobs did not finish in time")
		return nil
	}
	steps := []Step{{Name: "show", Cmd: []string{"cat", "README"}}}
	const cloneURL = "https://github.com/octocat/Hello-World.git"

	must("CheckAuth", c.CheckAuth())

	// Job from a clone URL
	must("CreateJobWithURL", c.CreateJobWithURL(cloneURL, steps))
	jobs := waitJobs()
	if jobs[0].Status != "success" {
		t.Fatalf("job status = %s, want success", jobs[0].Status)
	}
	zip, err := c.DownloadJobArtifact(jobs[0].ID)
	must("DownloadJobArtifact", err)
	if len(zip) == 0 {
		t.Fatal("empty artifact")
	}
	must("DeleteJobByID", c.DeleteJobByID(jobs[0].ID))

	// Job from an organization repository
	post("/organizations", map[string]string{"name": "it-org"})
	orgs, err := c.GetOrganizations()
	must("GetOrganizations", err)
	post("/organizations/"+orgs[0].ID+"/repositories", map[string]string{"name": "hello", "repo": cloneURL})
	repos, err := c.GetRepositories(orgs[0].ID)
	must("GetRepositories", err)
	must("CreateJob", c.CreateJob(orgs[0].ID, repos[0].ID, steps))
	waitJobs()
	repoJobs, err := c.GetJobs(orgs[0].ID, repos[0].ID)
	must("GetJobs", err)
	if len(repoJobs) != 1 {
		t.Fatalf("GetJobs returned %d jobs, want 1", len(repoJobs))
	}
	must("DeleteJob", c.DeleteJob(orgs[0].ID, repos[0].ID, repoJobs[0].ID))

	// SSH keys
	_, err = c.CreateSSHKey("it-key", "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----\n")
	must("CreateSSHKey", err)
	keys, err := c.GetSSHKeys()
	must("GetSSHKeys", err)
	must("DeleteSSHKey", c.DeleteSSHKey(keys[0].ID))
}
