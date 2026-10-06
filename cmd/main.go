package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaspose/goci-cli/internal/api"
	"github.com/lucaspose/goci-cli/internal/config"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("gopherci %s (%s)\n", version, commit)
			return
		case "-h", "--help", "help":
			fmt.Println("gopherci — terminal client for GopherCI\n\nUsage:\n  gopherci            start the interactive UI\n  gopherci --version  print the version")
			return
		}
	}

	emailInput := textinput.New()
	emailInput.Placeholder = "email"
	emailInput.Focus()

	passwordInput := textinput.New()
	passwordInput.Placeholder = "password"
	passwordInput.EchoMode = textinput.EchoPassword

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{APIURL: config.DefaultAPIURL}
	}

	// Avoid carrying over test fixture values into runtime config.
	if strings.TrimSpace(cfg.APIURL) == "" || cfg.APIURL == config.TestSentinelAPIURL {
		cfg.APIURL = config.DefaultAPIURL
		if cfg.Token == "tok" {
			cfg.Token = ""
		}
		if cfg.GitHubToken == "tok" {
			cfg.GitHubToken = ""
		}
		_ = config.Save(cfg)
	}
	if url := strings.TrimSpace(os.Getenv(config.APIURLEnv)); url != "" {
		cfg.APIURL = url
	}

	initialScreen := screenLogin
	if err == nil && (cfg.Token != "" || cfg.GitHubToken != "") {
		initialScreen = screenMenu
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = loadingStyle

	m := model{
		screen:    initialScreen,
		config:    cfg,
		apiClient: api.NewClient(cfg.APIURL, cfg.Token),
		spinner:   sp,
		login: loginState{
			inputs: []textinput.Model{emailInput, passwordInput},
		},
		jobs: jobsState{
			pageSize: defaultJobsPageSize,
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
