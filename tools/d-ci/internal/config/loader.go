package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string

	GitLabToken string
	GitLabURL   string // Для Self-hosted инстансов

	JenkinsURL  string
	JenkinsUser string
	JenkinsKey  string
}

// Load ищет .env файл локально или в ~/.config/devos/
func Load() (*Config, error) {
	_ = godotenv.Load("d-ci.env")
	// 1. Пытаемся загрузить локальный .env
	_ = godotenv.Load(".env")

	// 2. Пытаемся загрузить глобальный конфиг ~/.config/devos/d-ci.env
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".config", "devos", "d-ci.env")
		_ = godotenv.Load(globalPath)
	}

	cfg := &Config{
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		GitHubOwner: os.Getenv("GITHUB_OWNER"),
		GitHubRepo:  os.Getenv("GITHUB_REPO"),

		GitLabToken: os.Getenv("GITLAB_TOKEN"),
		GitLabURL:   os.Getenv("GITLAB_URL"), // Default: https://gitlab.com

		JenkinsURL:  os.Getenv("JENKINS_URL"),
		JenkinsUser: os.Getenv("JENKINS_USER"),
		JenkinsKey:  os.Getenv("JENKINS_API_TOKEN"),
	}

	return cfg, nil
}

// CreateTemplate создает файл-шаблон, если его нет
func CreateTemplate() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "devos")
	file := filepath.Join(dir, "d-ci.env")

	if _, err := os.Stat(file); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
		content := `# DevOS d-ci Configuration
# Раскомментируйте и заполните нужные секции

# --- GitHub ---
# GITHUB_TOKEN=ghp_your_token_here
# GITHUB_OWNER=devos-os
# GITHUB_REPO=d-ci

# --- GitLab ---
# GITLAB_TOKEN=glpat_your_token
# GITLAB_URL=https://gitlab.com

# --- Jenkins ---
# JENKINS_URL=https://jenkins.example.com
# JENKINS_USER=admin
# JENKINS_API_TOKEN=your_api_token
`
		os.WriteFile(file, []byte(content), 0644)
		fmt.Printf("ℹ️  Created default config at: %s\n", file)
	}
}