package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetRepoRoot находит абсолютный путь к корню репозитория
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// GetChangedFiles возвращает список АБСОЛЮТНЫХ путей к файлам
func GetChangedFiles(isCI bool, baseBranch string) ([]string, error) {
	root, err := GetRepoRoot()
	if err != nil {
		return nil, err
	}

	var files []string

	if isCI {
		// --- CI MODE ---
		// Сравниваем HEAD с базовой веткой
		if baseBranch == "" {
			baseBranch = "origin/main" // Default fallback
		}
		// Используем 'git diff --diff-filter=d' чтобы исключить удаленные файлы
		out, err := runGit(root, "diff", "--name-only", "--diff-filter=d", baseBranch+"...HEAD")
		if err != nil {
			return nil, err
		}
		files = parseOutput(out)
	} else {
		// --- LOCAL MODE (Pre-commit / Audit) ---
		// Нам нужно собрать ВСЕ изменения, над которыми работает разработчик
		
		// 1. Unstaged (Modified)
		out1, _ := runGit(root, "diff", "--name-only", "--diff-filter=d")
		files = append(files, parseOutput(out1)...)

		// 2. Staged (Ready to commit)
		out2, _ := runGit(root, "diff", "--name-only", "--cached", "--diff-filter=d")
		files = append(files, parseOutput(out2)...)

		// 3. Untracked (New files) - КРИТИЧНО ВАЖНО
		out3, _ := runGit(root, "ls-files", "--others", "--exclude-standard")
		files = append(files, parseOutput(out3)...)
	}

	// Превращаем в абсолютные пути и удаляем дубликаты
	uniquePaths := make(map[string]bool)
	var absFiles []string

	for _, f := range files {
		if f == "" { continue }
		absPath := filepath.Join(root, f)
		if !uniquePaths[absPath] {
			uniquePaths[absPath] = true
			absFiles = append(absFiles, absPath)
		}
	}

	return absFiles, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir // Важно: выполняем команды от корня репо
	out, err := cmd.Output()
	return string(out), err
}

func parseOutput(raw string) []string {
	return strings.Split(strings.TrimSpace(raw), "\n")
}