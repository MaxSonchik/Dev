package docker

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv" // <--- ДОБАВЛЕН ИМПОРТ
	"strings"
)

type DockerData struct {
	Found       bool
	Stages      []BuildStage
	ComposeSvcs []string
	Compose     []ComposeService
}

type BuildStage struct {
	Name  string
	Base  string
	Steps []string
}

type ComposeService struct {
	Name      string
	Image     string
	DependsOn []string
}

func Analyze(root string) DockerData {
	d := DockerData{}
	
	// Dockerfile Analysis
	path := filepath.Join(root, "Dockerfile")
	if file, err := os.Open(path); err == nil {
		d.Found = true
		scanner := bufio.NewScanner(file)
		var current *BuildStage
		stageCount := 0
		
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "FROM") {
				stageCount++
				parts := strings.Fields(line)
				if len(parts) < 2 { continue }
				
				base := parts[1]
				
				// ИСПРАВЛЕНИЕ ЗДЕСЬ: используем strconv.Itoa вместо string()
				name := "Stage " + strconv.Itoa(stageCount)
				
				if len(parts) >= 4 && strings.ToLower(parts[2]) == "as" {
					name = parts[3]
				}
				d.Stages = append(d.Stages, BuildStage{Name: name, Base: base, Steps: []string{}})
				current = &d.Stages[len(d.Stages)-1]
			} else if current != nil && (strings.HasPrefix(line, "RUN") || strings.HasPrefix(line, "COPY")) {
				current.Steps = append(current.Steps, line)
			}
		}
		file.Close()
	}

	// Docker Compose Analysis
	dcPath := filepath.Join(root, "docker-compose.yml")
	if file, err := os.Open(dcPath); err == nil {
		d.Found = true
		scanner := bufio.NewScanner(file)
		var currentSvc *ComposeService
		indent := 0
		
		for scanner.Scan() {
			rawLine := scanner.Text()
			line := strings.TrimSpace(rawLine)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			
			currIndent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))
			
			// Detect service
			if currIndent == 2 && strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "version") && !strings.HasPrefix(line, "services") {
				name := strings.TrimSuffix(line, ":")
				d.Compose = append(d.Compose, ComposeService{Name: name})
				currentSvc = &d.Compose[len(d.Compose)-1]
				indent = 2
				continue
			}

			if currentSvc != nil && currIndent > indent {
				if strings.HasPrefix(line, "image:") {
					currentSvc.Image = strings.TrimPrefix(line, "image: ")
				}
				// Простой парсинг depends_on
				if strings.TrimSpace(line) == "depends_on:" {
					// В следующих строках будем искать зависимости (упрощенно)
				}
				if strings.HasPrefix(line, "- ") && currIndent > indent+2 {
					// Это может быть элемент списка (например, depends_on)
					// Для MVP сложный парсинг YAML вручную опустим, полагаясь на регулярки в других модулях
				}
			}
		}
		file.Close()
	}

	return d
}