package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devos-os/d-ci/internal/config"
	"github.com/devos-os/d-ci/internal/domain"
	"github.com/devos-os/d-ci/internal/providers"
	"github.com/devos-os/d-ci/internal/ui"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "d-ci",
		Short: "DevOS CI Monitor",
		Long:  "Universal CI/CD dashboard. Config file: ~/.config/devos/d-ci.env",
		Run:   run,
	}
	
	// Оставляем флаг --mock для тестов
	rootCmd.PersistentFlags().Bool("mock", false, "Use mock data instead of real API")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	// 0. Создаем шаблон конфига, если его нет
	config.CreateTemplate()

	// 1. Загружаем конфиг
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
	}

	useMock, _ := cmd.Flags().GetBool("mock")
	var provider domain.Provider

	// 2. Логика выбора провайдера
	if useMock {
		fmt.Println("🔮 Using Mock Provider")
		provider = providers.NewMockProvider()
	} else if cfg.GitHubToken != "" && cfg.GitHubRepo != "" {
		// Если есть токен GitHub -> используем его
		// (В будущем можно добавить меню выбора, если настроены оба)
		fmt.Println("octocat Using GitHub Provider...")
		gh, err := providers.NewGitHubProvider(cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubToken)
		if err != nil {
			fmt.Printf("❌ GitHub init failed: %v\n", err)
			os.Exit(1)
		}
		provider = gh
	} else {
		// Fallback
		fmt.Println("⚠️  No providers configured in ~/.config/devos/d-ci.env")
		fmt.Println("🔮 Switching to Mock Mode for demonstration...")
		provider = providers.NewMockProvider()
	}

	// 3. Запуск UI
	model := ui.NewModel(provider)
	p := tea.NewProgram(model, tea.WithAltScreen()) // AltScreen = полноэкранный режим

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}