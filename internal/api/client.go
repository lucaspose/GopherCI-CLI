package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	ID        string   `json:"id"`
	CloneURL    string   `json:"clone_url"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	Logs      []string `json:"logs"`
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

// ExchangeGitHubToken swaps a GitHub OAuth token for a GopherCI access token.
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
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	if err != nil {
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
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("create job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateJobWithURL(cloneURL string, steps []Step) error {
    body, err := json.Marshal(map[string]interface{}{
		"clone_url": cloneURL,
		"steps": steps,
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
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("create job failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetGitHubRepositories(githubToken string) ([]GitHubRepo, error) {
    url := c.BaseURL + "/auth/github/repositories?token=" + githubToken
	req, err := http.NewRequest("GET", url, nil)
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get jobs failed: %d", resp.StatusCode)
	}
	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// CheckAuth verifies the stored token is still valid by hitting an
// authenticated endpoint. Returns an error containing "401" on expiry so
// the existing redirect-to-login logic in Update() triggers automatically.
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