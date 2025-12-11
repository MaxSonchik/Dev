package internal

import (
	"fmt"
	"sync"

	"github.com/devos-os/d-guard/internal/core"
	"github.com/devos-os/d-guard/internal/git"
	"github.com/devos-os/d-guard/internal/modules/code"      // Наш нативный
	"github.com/devos-os/d-guard/internal/modules/container" // Наш нативный
	"github.com/devos-os/d-guard/internal/modules/external"  // Trivy (старый)
	"github.com/devos-os/d-guard/internal/modules/secrets"   // Наш нативный (Fallback)
	"github.com/devos-os/d-guard/internal/tools"             // Новые (Gitleaks, Semgrep)
)

func RunAll(cfg core.Config) []core.Issue {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allIssues []core.Issue

	// 1. Определяем файлы
	root, _ := git.GetRepoRoot()
	var files []string
	if cfg.ScanAll {
		// Для scan-all передаем пустой список, инструменты сами просканируют папку
		files = []string{} 
	} else {
		base := cfg.BaseBranch
		if cfg.IsCI && base == "" { base = "origin/main" }
		files, _ = git.GetChangedFiles(cfg.IsCI, base)
		if len(files) == 0 { return nil }
	}

	fmt.Printf("🚀 Orchestrating security scan on %s (Parallel execution)...\n", root)

	// Хелпер для запуска
	run := func(name string, fn func() []core.Issue) {
		defer wg.Done()
		fmt.Printf("  ⏳ Starting %s...\n", name)
		res := fn()
		mu.Lock()
		allIssues = append(allIssues, res...)
		mu.Unlock()
		if len(res) > 0 {
			fmt.Printf("  🔴 %s found %d issues\n", name, len(res))
		} else {
			fmt.Printf("  ✅ %s clean\n", name)
		}
	}

	// --- Запуск потоков ---
	
	// 1. Gitleaks (Secrets)
	wg.Add(1)
	go run("Gitleaks", func() []core.Issue {
		res := tools.RunGitleaks(root, files)
		if res == nil { // Fallback to native if not installed
			return secrets.Scan(files)
		}
		return res
	})

	// 2. Semgrep (SAST)
	wg.Add(1)
	go run("Semgrep", func() []core.Issue {
		return tools.RunSemgrep(root, files)
	})

	// 3. Trivy (SCA & IaC)
	wg.Add(1)
	go run("Trivy", func() []core.Issue {
		return external.RunTrivyFs(root) // Trivy лучше работает по всей папке
	})

	// 4. Native Docker (Runtime + Static)
	wg.Add(1)
	go run("Native Docker", func() []core.Issue {
		return container.Scan(files)
	})

	// 5. Native Code Quality
	wg.Add(1)
	go run("Code Quality", func() []core.Issue {
		return code.Scan(files)
	})

	wg.Wait()
	return allIssues
}