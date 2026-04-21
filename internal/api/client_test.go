package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lucaspose/goci-cli/internal/api"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-token")
}

func authHeader(r *http.Request) string {
	return r.Header.Get("Authorization")
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/login" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["email"] != "user@test.com" || body["password"] != "secret" {
			t.Errorf("unexpected body: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.LoginResponse{AccessToken: "tok123", ExpiresIn: 3600})
	})

	resp, err := client.Login("user@test.com", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.AccessToken != "tok123" {
		t.Errorf("expected tok123, got %s", resp.AccessToken)
	}
}

func TestLogin_BadCredentials(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.Login("bad", "creds")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

// ── GetOrganizations ──────────────────────────────────────────────────────────

func TestGetOrganizations_Success(t *testing.T) {
	orgs := []api.Organization{{ID: "org1", Name: "my-org"}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if authHeader(r) != "Bearer test-token" {
			t.Errorf("missing auth header")
		}
		json.NewEncoder(w).Encode(orgs)
	})

	result, err := client.GetOrganizations()
	if err != nil {
		t.Fatalf("GetOrganizations: %v", err)
	}
	if len(result) != 1 || result[0].ID != "org1" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestGetOrganizations_Unauthorized(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.GetOrganizations()
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── GetRepositories ───────────────────────────────────────────────────────────

func TestGetRepositories_Success(t *testing.T) {
	repos := []api.Repository{{ID: "r1", Name: "backend"}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/org1/repositories" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(repos)
	})

	result, err := client.GetRepositories("org1")
	if err != nil {
		t.Fatalf("GetRepositories: %v", err)
	}
	if len(result) != 1 || result[0].Name != "backend" {
		t.Errorf("unexpected result: %v", result)
	}
}

// ── GetJobs ───────────────────────────────────────────────────────────────────

func TestGetJobs_Success(t *testing.T) {
	jobs := []api.Job{{ID: "job1", Status: "success", CreatedAt: time.Now()}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/org1/repositories/repo1/jobs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(jobs)
	})

	result, err := client.GetJobs("org1", "repo1")
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(result) != 1 || result[0].ID != "job1" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestGetJobs_Unauthorized(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.GetJobs("org1", "repo1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── GetAllJobs ────────────────────────────────────────────────────────────────

func TestGetAllJobs_Success(t *testing.T) {
	jobs := []api.Job{{ID: "j1", Status: "running"}, {ID: "j2", Status: "failed"}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(jobs)
	})

	result, err := client.GetAllJobs()
	if err != nil {
		t.Fatalf("GetAllJobs: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(result))
	}
}

func TestDownloadJobArtifact_Success(t *testing.T) {
	artifact := []byte("zip-bytes")
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/jobs/job1/artifact" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if authHeader(r) != "Bearer test-token" {
			t.Errorf("missing auth header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifact)
	})

	result, err := client.DownloadJobArtifact("job1")
	if err != nil {
		t.Fatalf("DownloadJobArtifact: %v", err)
	}
	if string(result) != string(artifact) {
		t.Fatalf("unexpected artifact content: %q", string(result))
	}
}

func TestDownloadJobArtifact_Unauthorized(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := client.DownloadJobArtifact("job1"); err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// ── CreateJob ─────────────────────────────────────────────────────────────────

func TestCreateJob_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if authHeader(r) != "Bearer test-token" {
			t.Errorf("missing auth header")
		}
		w.WriteHeader(http.StatusAccepted)
	})

	err := client.CreateJob("org1", "repo1", []api.Step{{Name: "test", Cmd: []string{"go", "test"}}})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
}

func TestCreateJob_BadRequest(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	err := client.CreateJob("org1", "repo1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── CreateJobWithURL ──────────────────────────────────────────────────────────

func TestCreateJobWithURL_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["clone_url"] != "https://github.com/user/repo.git" {
			t.Errorf("unexpected clone_url: %v", body["clone_url"])
		}
		w.WriteHeader(http.StatusAccepted)
	})

	err := client.CreateJobWithURL("https://github.com/user/repo.git", []api.Step{{Name: "build", Cmd: []string{"make"}}})
	if err != nil {
		t.Fatalf("CreateJobWithURL: %v", err)
	}
}

// ── DeleteJob ─────────────────────────────────────────────────────────────────

func TestDeleteJob_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/organizations/org1/repositories/repo1/jobs/job1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteJob("org1", "repo1", "job1")
	if err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	err := client.DeleteJob("org1", "repo1", "bad-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── DeleteJobByID ─────────────────────────────────────────────────────────────

func TestDeleteJobByID_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/job42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteJobByID("job42")
	if err != nil {
		t.Fatalf("DeleteJobByID: %v", err)
	}
}

// ── SSH Keys ──────────────────────────────────────────────────────────────────

func TestGetSSHKeys_Success(t *testing.T) {
	keys := []api.SSHKey{{ID: "k1", Name: "deploy-key", Fingerprint: "SHA256:abc"}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ssh-keys" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(keys)
	})

	result, err := client.GetSSHKeys()
	if err != nil {
		t.Fatalf("GetSSHKeys: %v", err)
	}
	if len(result) != 1 || result[0].Name != "deploy-key" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestCreateSSHKey_Success(t *testing.T) {
	created := api.SSHKey{ID: "k2", Name: "home", Fingerprint: "SHA256:xyz"}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/ssh-keys" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "home" {
			t.Errorf("unexpected name: %s", body["name"])
		}
		if body["private_key"] != "PRIVATE-KEY" {
			t.Errorf("unexpected private_key: %s", body["private_key"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	result, err := client.CreateSSHKey("home", "PRIVATE-KEY")
	if err != nil {
		t.Fatalf("CreateSSHKey: %v", err)
	}
	if result.ID != "k2" {
		t.Errorf("unexpected ID: %s", result.ID)
	}
}

func TestCreateSSHKey_Success_EmptyBody(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	if _, err := client.CreateSSHKey("home", "PRIVATE-KEY"); err != nil {
		t.Fatalf("CreateSSHKey with empty body: %v", err)
	}
}

func TestDeleteSSHKey_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/ssh-keys/k1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteSSHKey("k1")
	if err != nil {
		t.Fatalf("DeleteSSHKey: %v", err)
	}
}

// ── CheckAuth ─────────────────────────────────────────────────────────────────

func TestCheckAuth_Valid(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if authHeader(r) != "Bearer test-token" {
			t.Errorf("missing auth header")
		}
		json.NewEncoder(w).Encode([]api.Organization{})
	})

	err := client.CheckAuth()
	if err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
}

func TestCheckAuth_Expired(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	err := client.CheckAuth()
	if err == nil {
		t.Fatal("expected 401 error")
	}
}

// ── ExchangeGitHubToken ───────────────────────────────────────────────────────

func TestExchangeGitHubToken_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/github/exchange" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["github_token"] != "ghp_abc" {
			t.Errorf("unexpected github_token: %s", body["github_token"])
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "goci_tok"})
	})

	tok, err := client.ExchangeGitHubToken("ghp_abc")
	if err != nil {
		t.Fatalf("ExchangeGitHubToken: %v", err)
	}
	if tok != "goci_tok" {
		t.Errorf("expected goci_tok, got %s", tok)
	}
}

// ── GetGitHubRepositories ─────────────────────────────────────────────────────

func TestGetGitHubRepositories_Success(t *testing.T) {
	repos := []api.GitHubRepo{{Name: "myrepo", FullName: "user/myrepo", CloneURL: "https://github.com/user/myrepo.git"}}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/github/repositories" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") != "ghp_abc" {
			t.Errorf("unexpected token param")
		}
		json.NewEncoder(w).Encode(repos)
	})

	result, err := client.GetGitHubRepositories("ghp_abc")
	if err != nil {
		t.Fatalf("GetGitHubRepositories: %v", err)
	}
	if len(result) != 1 || result[0].Name != "myrepo" {
		t.Errorf("unexpected result: %v", result)
	}
}
