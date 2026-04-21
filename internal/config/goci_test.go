package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucaspose/goci-cli/internal/config"
)

func writeTempGoci(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), ".goci")
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatalf("write temp .goci: %v", err)
	}
	return f
}

func TestLoadGociConfigFrom_Valid(t *testing.T) {
	path := writeTempGoci(t, `
[pipeline]
name = "my-app"

[[steps]]
name = "install"
cmd  = ["npm", "install"]

[[steps]]
name = "test"
cmd  = ["npm", "test"]
`)

	cfg, err := config.LoadGociConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadGociConfigFrom: %v", err)
	}
	if cfg.Pipeline.Name != "my-app" {
		t.Errorf("Pipeline.Name: got %q, want %q", cfg.Pipeline.Name, "my-app")
	}
	if len(cfg.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(cfg.Steps))
	}
	if cfg.Steps[0].Name != "install" {
		t.Errorf("step 0 name: got %q, want install", cfg.Steps[0].Name)
	}
	if len(cfg.Steps[0].Cmd) != 2 || cfg.Steps[0].Cmd[0] != "npm" {
		t.Errorf("step 0 cmd: unexpected %v", cfg.Steps[0].Cmd)
	}
}

func TestLoadGociConfigFrom_Missing(t *testing.T) {
	_, err := config.LoadGociConfigFrom("/nonexistent/path/.goci")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadGociConfigFrom_EmptySteps(t *testing.T) {
	path := writeTempGoci(t, `
[pipeline]
name = "empty"
`)

	cfg, err := config.LoadGociConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadGociConfigFrom: %v", err)
	}
	if len(cfg.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(cfg.Steps))
	}
}

func TestLoadGociConfigFrom_MultipleSteps(t *testing.T) {
	path := writeTempGoci(t, `
[[steps]]
name = "lint"
cmd  = ["golangci-lint", "run"]

[[steps]]
name = "test"
cmd  = ["go", "test", "./..."]

[[steps]]
name = "build"
cmd  = ["go", "build", "./..."]
`)

	cfg, err := config.LoadGociConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadGociConfigFrom: %v", err)
	}
	if len(cfg.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(cfg.Steps))
	}
	if cfg.Steps[2].Name != "build" {
		t.Errorf("step 2 name: got %q, want build", cfg.Steps[2].Name)
	}
}

func TestLoadGociConfigFrom_InvalidTOML(t *testing.T) {
	path := writeTempGoci(t, `
[pipeline
name = "broken"
`)

	_, err := config.LoadGociConfigFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadGociConfigFrom_NoPipelineName(t *testing.T) {
	path := writeTempGoci(t, `
[[steps]]
name = "test"
cmd  = ["go", "test"]
`)

	cfg, err := config.LoadGociConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadGociConfigFrom: %v", err)
	}
	if cfg.Pipeline.Name != "" {
		t.Errorf("expected empty pipeline name, got %q", cfg.Pipeline.Name)
	}
	if len(cfg.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(cfg.Steps))
	}
}
