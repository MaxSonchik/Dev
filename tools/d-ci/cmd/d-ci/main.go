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
	f, err := tea.LogToFile("d-ci.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

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
	useMock, _ := cmd.Flags().GetBool("mock")

	// Карта: "owner/repo" -> Provider
	providersMap := make(map[string]domain.Provider)
	var activeProviders []domain.Provider // Для запуска подписки

	if useMock {
		log.Println("⚠️ Using Mock Provider")
		m1 := providers.NewMockProvider("mock/repo-1")
		m2 := providers.NewMockProvider("mock/repo-2")
		
		providersMap["mock/repo-1"] = m1
		providersMap["mock/repo-2"] = m2
		activeProviders = append(activeProviders, m1, m2)
	} else {
		// --- GitHub ---
		if cfg.GitHubToken != "" && len(cfg.GitHubRepos) > 0 {
			for _, repo := range cfg.GitHubRepos {
				fullRepoName := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
				log.Printf("🔌 Init GitHub: %s", fullRepoName)
				
				gh, err := providers.NewGitHubProvider(repo.Owner, repo.Name, cfg.GitHubToken)
				if err == nil {
					providersMap[fullRepoName] = gh
					activeProviders = append(activeProviders, gh)
				} else {
					log.Printf("❌ Failed to init GitHub %s: %v", fullRepoName, err)
				}
			}
		}

		// --- GitLab ---
		if cfg.GitLabToken != "" && len(cfg.GitLabRepos) > 0 {
			for _, repo := range cfg.GitLabRepos {
				fullRepoName := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
				log.Printf("🔌 Init GitLab: %s", fullRepoName)
				
				gl := providers.NewGitLabProvider(cfg.GitLabURL, cfg.GitLabToken, fullRepoName)
				if err := gl.Ping(); err != nil {
					log.Printf("❌ GitLab Error (%s): %v", fullRepoName, err)
				} else {
					providersMap[fullRepoName] = gl
					activeProviders = append(activeProviders, gl)
				}
			}
		}
	}

	if len(activeProviders) == 0 {
		fmt.Println("❌ No providers configured. Check your .env file or use --mock")
		return
	}

	// Fan-in: Объединяем каналы событий
	aggChannel := make(chan domain.PipelineEvent)
	for _, p := range activeProviders {
		go func(prov domain.Provider) {
			for event := range prov.Subscribe() {
				aggChannel <- event
			}
		}(p)
	}

	// Передаем MAP в UI
	model := ui.NewModel(providersMap, aggChannel)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}