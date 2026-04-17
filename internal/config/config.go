package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
