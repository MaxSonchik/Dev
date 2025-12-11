package tools

import (
	"fmt"
	//"io"
	//"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	// Версии инструментов (фиксируем для стабильности)
	gitleaksVer = "8.18.1"
	semgrepVer  = "1.52.0" // Используем open-source ядро
)

// EnsureTool проверяет наличие инструмента, или скачивает его
func EnsureTool(name string) (string, error) {
	// 1. Сначала ищем в PATH системы
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// 2. Ищем в локальном кэше DevOS
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cache", "devos", "bin")
	toolPath := filepath.Join(cacheDir, name)

	if _, err := os.Stat(toolPath); err == nil {
		return toolPath, nil
	}

	// 3. Скачиваем, если нет
	fmt.Printf("⬇️  Tool '%s' not found. Downloading automatically...\n", name)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	return downloadTool(name, toolPath)
}

func downloadTool(name, dest string) (string, error) {
	arch := runtime.GOARCH
	//osName := runtime.GOOS

	switch name {
	case "gitleaks":
		if arch == "amd64" { arch = "x64" }

		return "", fmt.Errorf("please install manually: curl -sSfL https://github.com/gitleaks/gitleaks/releases/download/v%s/gitleaks_%s_x64.tar.gz | tar xz", gitleaksVer, gitleaksVer)
	
	case "semgrep":
		// Semgrep устанавливается через pip, это сложнее для binary drop.
		// Но мы можем проверить python environment.
		return "", fmt.Errorf("please install manually: pip install semgrep")
	}

	return "", fmt.Errorf("unknown tool")
}