package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucaspose/goci-cli/internal/config"
)

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{
		Token:       "my-token",
		APIURL:      "http://example.com:8080",
		GitHubToken: "ghp_abc123",
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token: got %q, want %q", loaded.Token, cfg.Token)
	}
	if loaded.APIURL != cfg.APIURL {
		t.Errorf("APIURL: got %q, want %q", loaded.APIURL, cfg.APIURL)
	}
	if loaded.GitHubToken != cfg.GitHubToken {
		t.Errorf("GitHubToken: got %q, want %q", loaded.GitHubToken, cfg.GitHubToken)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".goci-cli")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("not valid json{{"), 0600)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &config.Config{APIURL: "http://localhost:8080"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(tmp, ".goci-cli", "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("config file not created at %s", path)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{Token: "secret"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".goci-cli", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}
