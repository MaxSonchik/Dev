package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type RepoConfig struct {
	Owner string
	Name  string
}

type Config struct {
	GitHubToken string
	GitLabToken string
	GitLabURL   string

	GitHubRepos []RepoConfig
	GitLabRepos []RepoConfig
}

func Load() (*Config, error) {
	// 1. Загружаем переменные из файлов
	_ = godotenv.Load("d-ci.env")
	_ = godotenv.Load(".env")
	
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "devos", "d-ci.env")
	_ = godotenv.Load(globalPath)

	cfg := &Config{
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		GitLabToken: os.Getenv("GITLAB_TOKEN"),
		GitLabURL:   os.Getenv("GITLAB_URL"),
	}

	if cfg.GitLabURL == "" {
		cfg.GitLabURL = "https://gitlab.com"
	}

	// 2. Парсим НОВЫЙ формат (списки)
	cfg.GitHubRepos = parseRepos(os.Getenv("GITHUB_REPOS"))
	cfg.GitLabRepos = parseRepos(os.Getenv("GITLAB_REPOS"))

	// 3. Обратная совместимость (СТАРЫЙ формат)
	// Если список пуст, но заданы OWNER/REPO, добавляем их вручную
	if len(cfg.GitHubRepos) == 0 {
		owner := os.Getenv("GITHUB_OWNER")
		repo := os.Getenv("GITHUB_REPO")
		if owner != "" && repo != "" {
			cfg.GitHubRepos = append(cfg.GitHubRepos, RepoConfig{
				Owner: owner,
				Name:  repo,
			})
		}
	}

	return cfg, nil
}

func parseRepos(input string) []RepoConfig {
	var result []RepoConfig
	if input == "" {
		return result
	}
	
	parts := strings.Split(input, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Формат: owner/repo
		idx := strings.LastIndex(p, "/")
		if idx != -1 {
			result = append(result, RepoConfig{
				Owner: p[:idx],
				Name:  p[idx+1:],
			})
		}
	}
	return result
}