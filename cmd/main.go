package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

func main() {
	emailInput := textinput.New()
	emailInput.Placeholder = "email"
	emailInput.Focus()

	passwordInput := textinput.New()
	passwordInput.Placeholder = "password"
	passwordInput.EchoMode = textinput.EchoPassword

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{APIURL: "http://localhost:8080"}
	}

	// Avoid carrying over test fixture values into runtime config.
	if strings.TrimSpace(cfg.APIURL) == "" || cfg.APIURL == "http://test" {
		cfg.APIURL = "http://localhost:8080"
		if cfg.Token == "tok" {
			cfg.Token = ""
		}
		if cfg.GitHubToken == "tok" {
			cfg.GitHubToken = ""
		}
		_ = config.Save(cfg)
	}

	initialScreen := screenLogin
	if err == nil && (cfg.Token != "" || cfg.GitHubToken != "") {
		initialScreen = screenMenu
	}

	m := model{
		screen:    initialScreen,
		config:    cfg,
		apiClient: api.NewClient(cfg.APIURL, cfg.Token),
		login: loginState{
			inputs: []textinput.Model{emailInput, passwordInput},
		},
		jobs: jobsState{
			pageSize: defaultJobsPageSize,
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
