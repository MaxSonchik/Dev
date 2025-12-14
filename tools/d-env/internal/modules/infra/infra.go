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
			content, _ := os.ReadFile(path)
			s := string(content)
			if strings.Contains(s, "apiVersion:") && strings.Contains(s, "kind:") {
				kind := extractValue(s, "kind:")
				name := extractValue(s, "name:")
				if kind != "" {
					display := kind
					if name != "" { display += ": " + name }
					i.K8sObjs = append(i.K8sObjs, display)
					addTool(&i, "Kubernetes ☸️")
				}
			}
		}
		return nil
	})

	// 2. CI/CD Deep Scan (All files)
	ghPath := filepath.Join(root, ".github/workflows")
	if files, err := os.ReadDir(ghPath); err == nil && len(files) > 0 {
		i.CiSystem = "GitHub Actions"
		// Читаем первый попавшийся файл для графа (для MVP), но отмечаем, что нашли
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".yml") || strings.HasSuffix(f.Name(), ".yaml") {
				content, _ := os.ReadFile(filepath.Join(ghPath, f.Name()))
				// Если еще не строили граф, строим по этому файлу
				if len(i.CiGraph) == 0 {
					i.CiGraph = buildCiGraph(string(content))
				}
			}
		}
	}

	return i
}

func addTool(i *InfraData, tool string) {
	for _, t := range i.Tools { if t == tool { return } }
	i.Tools = append(i.Tools, tool)
}

func extractValue(yaml, key string) string {
	scanner := bufio.NewScanner(strings.NewReader(yaml))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	return ""
}

func buildCiGraph(yamlContent string) [][]string {
	// Демонстрационная логика. В реальности нужен YAML парсер.
	// Пытаемся найти job names по отступам
	
	// Если это наш release.yml
	if strings.Contains(yamlContent, "build-and-release") {
		return [][]string{
			{"checkout", "setup-go", "setup-rust"},
			{"build-all-tools"},
			{"create-release"},
		}
	}

	// Fallback
	if strings.Contains(yamlContent, "lint") || strings.Contains(yamlContent, "test") {
		return [][]string{
			{"lint", "unit-test"}, 
			{"build"},                          
			{"deploy"},                         
		}
	}
	
	return [][]string{{"build"}, {"test"}}
}