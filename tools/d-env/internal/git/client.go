package git

import (
	"os/exec"
	"strings"
)

type GitInfo struct {
	Branch      string
	Hash        string
	Message     string
	Author      string
	IsDirty     bool
	Initialized bool
}

func GetInfo() GitInfo {
	info := GitInfo{Initialized: false}

	// Проверка, есть ли git
	if _, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output(); err != nil {
		return info
	}
	info.Initialized = true

	// Ветка
	out, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	info.Branch = strings.TrimSpace(string(out))

	// Хэш и сообщение последнего коммита
	out, _ = exec.Command("git", "log", "-1", "--format=%h|%s|%an").Output()
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) >= 3 {
		info.Hash = parts[0]
		info.Message = parts[1]
		info.Author = parts[2]
	}

	// Статус (Dirty check)
	out, _ = exec.Command("git", "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		info.IsDirty = true
	}

	return info
}