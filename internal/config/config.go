package config

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultAPIURL matches a GopherCI server started locally (make up).
	DefaultAPIURL      = "http://localhost:8080"
	TestSentinelAPIURL = "http://test"

	// APIURLEnv overrides the configured API URL when set.
	APIURLEnv = "GOCI_API_URL"
)

type Config struct {
	Token       string `json:"token"`
	APIURL      string `json:"api_url"`
	GitHubToken string `json:"github_token"`
}

func Load() (*Config, error) {
	var cfg Config
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".goci-cli", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".goci-cli", "config.json")
	dir := filepath.Join(home, ".goci-cli")
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ValidateAPIURL rejects URLs that would send credentials in clear text:
// plain http:// is only allowed for a server on the local machine.
func ValidateAPIURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("invalid API URL")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return errors.New("use https:// for a remote server (http:// is only allowed for localhost)")
	default:
		return errors.New("API URL must start with https:// or http://localhost")
	}
}
