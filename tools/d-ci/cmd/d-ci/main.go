package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devos-os/d-ci/internal/config"
	"github.com/devos-os/d-ci/internal/domain"
	"github.com/devos-os/d-ci/internal/providers"
	"github.com/devos-os/d-ci/internal/ui"
	"github.com/spf13/cobra"
)

func main() {
	// --- НАСТРОЙКА ЛОГИРОВАНИЯ ---
	f, err := tea.LogToFile("d-ci.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()
	// -----------------------------

	var rootCmd = &cobra.Command{
		Use:   "d-ci",
		Short: "DevOS CI Monitor",
		Run:   run,
	}
	rootCmd.PersistentFlags().Bool("mock", false, "Use mock data")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	cfg, _ := config.Load()
	
	// ЛОГИРУЕМ КОНФИГУРАЦИЮ (Скрываем токен)
	tokenStatus := "MISSING"
	if len(cfg.GitHubToken) > 10 {
		tokenStatus = "PRESENT (" + cfg.GitHubToken[:4] + "...)"
	}
	log.Printf("🚀 Starting d-ci. Owner: %s, Repo: %s, Token: %s", cfg.GitHubOwner, cfg.GitHubRepo, tokenStatus)

	useMock, _ := cmd.Flags().GetBool("mock")
	var provider domain.Provider

	if useMock {
		provider = providers.NewMockProvider()
	} else if cfg.GitHubToken != "" {
		log.Println("🔌 Initializing GitHub Provider...")
		gh, err := providers.NewGitHubProvider(cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubToken)
		if err != nil {
			log.Printf("❌ GitHub Error: %v", err)
			os.Exit(1)
		}
		provider = gh
	} else {
		log.Println("⚠️ Config missing, falling back to Mock")
		provider = providers.NewMockProvider()
	}

	model := ui.NewModel(provider)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}