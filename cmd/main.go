package main

import (
	"fmt"

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

	initialScreen := screenLogin
	if err == nil && (cfg.Token != "" || cfg.GitHubToken != "") {
		initialScreen = screenMenu
	}

	m := model{
		screen:    initialScreen,
		config:    cfg,
		apiClient: api.NewClient(cfg.APIURL, cfg.Token),
		inputs:    []textinput.Model{emailInput, passwordInput},
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
