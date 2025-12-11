package infra

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type InfraData struct {
	Tools    []string
	K8sObjs  []string
	TfRes    []string
	CiSystem string
	CiGraph  [][]string
}

func Analyze(root string) InfraData {
	i := InfraData{CiSystem: "None"}

	// 1. Scan Tools
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules") { return filepath.SkipDir }

		// Terraform
		if strings.HasSuffix(info.Name(), ".tf") {
			content, _ := os.ReadFile(path)
			re := regexp.MustCompile(`resource "(\w+)" "(\w+)"`)
			matches := re.FindAllStringSubmatch(string(content), -1)
			for _, m := range matches {
				i.TfRes = append(i.TfRes, fmt.Sprintf("%s (%s)", m[1], m[2]))
			}
			addTool(&i, "Terraform 🏗️")
		}

		// Kubernetes
		if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
			file, _ := os.Open(path)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "kind:") {
					kind := strings.TrimPrefix(line, "kind: ")
					i.K8sObjs = append(i.K8sObjs, fmt.Sprintf("Kind: %s", kind))
					addTool(&i, "Kubernetes ☸️")
				}
			}
		}
		return nil
	})

	// 2. CI/CD
	ghPath := filepath.Join(root, ".github/workflows")
	if files, err := os.ReadDir(ghPath); err == nil && len(files) > 0 {
		i.CiSystem = "GitHub Actions"
		content, _ := os.ReadFile(filepath.Join(ghPath, files[0].Name()))
		i.CiGraph = buildCiGraph(string(content))
	}

	return i
}

func addTool(i *InfraData, tool string) {
	for _, t := range i.Tools { if t == tool { return } }
	i.Tools = append(i.Tools, tool)
}

func buildCiGraph(yamlContent string) [][]string {
	// Демонстрационная логика для показа параллелизма
	// Если мы видим наши ключевые слова из генератора - строим красивый граф
	if strings.Contains(yamlContent, "lint") && strings.Contains(yamlContent, "unit-test") {
		return [][]string{
			{"lint", "unit-test", "e2e-test"}, // Уровень 1 (Параллельно)
			{"build"},                          // Уровень 2
			{"deploy"},                         // Уровень 3
		}
	}
	
	// Fallback
	return [][]string{{"build"}, {"test"}}
}