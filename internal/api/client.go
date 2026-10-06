package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	httpClient *http.Client
}

type Repository struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Repo      string    `json:"repo"`
	OrgID     string    `json:"org_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type GitHubRepo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
	Private  bool   `json:"private"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type Step struct {
	Name string   `json:"name"`
	Cmd  []string `json:"cmd"`
}

type Job struct {
	ID         string     `json:"id"`
	CloneURL   string     `json:"clone_url"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Logs       []string   `json:"logs"`
}

// SSHKey represents a public SSH key registered on the GopherCI server.
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// HTTPClient exposes the configured HTTP client for use in package main commands.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) ExchangeGitHubToken(githubToken string) (string, error) {
	body, err := json.Marshal(map[string]string{"github_token": githubToken})
	if err != nil {
		return "", fmt.Errorf("exchange token error")
	}
	req, err := http.NewRequest("POST", c.BaseURL+"/auth/github/exchange", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("exchange token error")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange token error")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("exchange token failed: %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("exchange token error")
	}
	return result.AccessToken, nil
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.BaseURL+"/login", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: %d", resp.StatusCode)
	}
	var loginResp LoginResponse
	if err = json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, err
	}
	return &loginResp, nil
}

func (c *Client) GetJobs(orgID, repoID string) ([]Job, error) {
	url := c.BaseURL + "/organizations/" + orgID + "/repositories/" + repoID + "/jobs"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get jobs failed: %d", resp.StatusCode)
	}
	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *Client) GetOrganizations() ([]Organization, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/organizations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get organizations failed: %d", resp.StatusCode)
	}
	var organizations []Organization
	if err := json.NewDecoder(resp.Body).Decode(&organizations); err != nil {
		return nil, err
	}
	return organizations, nil
}

func (c *Client) GetRepositories(orgID string) ([]Repository, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/organizations/"+orgID+"/repositories", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get repositories failed: %d", resp.StatusCode)
	}
	var repositories []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repositories); err != nil {
		return nil, err
	}
	return repositories, nil
}

func (c *Client) DeleteJob(orgID, repoID, jobID string) error {
	url := c.BaseURL + "/organizations/" + orgID + "/repositories/" + repoID + "/jobs/" + jobID
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateJob(orgID, repoID string, steps []Step) error {
	url := c.BaseURL + "/organizations/" + orgID + "/repositories/" + repoID + "/jobs"
	body, err := json.Marshal(map[string]interface{}{
		"repo_id": repoID,
		"steps":   steps,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("create job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateJobWithURL(cloneURL string, steps []Step) error {
	body, err := json.Marshal(map[string]interface{}{
		"clone_url": cloneURL,
		"steps":     steps,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.BaseURL+"/jobs", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("create job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetGitHubRepositories(githubToken string) ([]GitHubRepo, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/auth/github/repositories", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Token", githubToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get github repositories failed: %d", resp.StatusCode)
	}
	var repositories []GitHubRepo
	if err = json.NewDecoder(resp.Body).Decode(&repositories); err != nil {
		return nil, err
	}
	return repositories, nil
}

func (c *Client) DeleteJobByID(jobID string) error {
	req, err := http.NewRequest("DELETE", c.BaseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetAllJobs() ([]Job, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get jobs failed: %d", resp.StatusCode)
	}
	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *Client) DownloadJobArtifact(jobID string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/jobs/"+jobID+"/artifact", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/zip,application/octet-stream")

	transport := c.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	downloadClient := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: transport,
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, buildHTTPError("download artifact failed", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("download artifact failed: empty artifact")
	}

	return body, nil
}

// ── SSH Keys ─────────────────────────────────────────────────────────────────

func (c *Client) GetSSHKeys() ([]SSHKey, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/ssh-keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get ssh-keys failed: %d", resp.StatusCode)
	}
	var keys []SSHKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (c *Client) CreateSSHKey(name, privateKey string) (*SSHKey, error) {
	body, err := json.Marshal(map[string]string{
		"name":        name,
		"private_key": privateKey,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.BaseURL+"/ssh-keys", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, buildHTTPError("create ssh-key failed", resp)
	}
	if resp.ContentLength == 0 {
		return &SSHKey{Name: name}, nil
	}
	var key SSHKey
	if err := json.NewDecoder(resp.Body).Decode(&key); err != nil {
		if errors.Is(err, io.EOF) {
			return &SSHKey{Name: name}, nil
		}
		return nil, err
	}
	return &key, nil
}

func buildHTTPError(prefix string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("%s: %d", prefix, resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		if s, ok := payload["error"].(string); ok && s != "" {
			msg = s
		} else if s, ok := payload["message"].(string); ok && s != "" {
			msg = s
		}
	}

	return fmt.Errorf("%s: %d (%s)", prefix, resp.StatusCode, msg)
}

func (c *Client) DeleteSSHKey(keyID string) error {
	req, err := http.NewRequest("DELETE", c.BaseURL+"/ssh-keys/"+keyID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete ssh-key failed: %d", resp.StatusCode)
	}
	return nil
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// CheckAuth verifies the stored token by hitting an authenticated endpoint.
// Returns an error containing "401" on expiry so the redirect-to-login logic triggers.
func (c *Client) CheckAuth() error {
	req, err := http.NewRequest("GET", c.BaseURL+"/organizations", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("401 unauthorized")
	}
	return nil
}
