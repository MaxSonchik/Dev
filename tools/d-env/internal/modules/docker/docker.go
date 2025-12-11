package docker

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type DockerData struct {
	Found       bool
	Dockerfile  DockerfileInfo
	Compose     ComposeInfo
}

type DockerfileInfo struct {
	Found  bool
	Stages []BuildStage
}

type BuildStage struct {
	ID        int
	Name      string // "builder"
	BaseImage string // "golang:1.21"
	IsFinal   bool
	Layers    []string // "RUN go build..."
	Links     []string // "COPY --from=builder"
}

type ComposeInfo struct {
	Found    bool
	Services []Service
}

type Service struct {
	Name      string
	Image     string
	BuildPath string
	Ports     []string
	Links     []string // depends_on names
}

func Analyze(root string) DockerData {
	d := DockerData{}

	// --- 1. Dockerfile Analysis ---
	dfPath := filepath.Join(root, "Dockerfile")
	if file, err := os.Open(dfPath); err == nil {
		d.Found = true
		d.Dockerfile.Found = true
		
		scanner := bufio.NewScanner(file)
		var current *BuildStage
		stageCount := 0

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") { continue }

			if strings.HasPrefix(line, "FROM") {
				stageCount++
				parts := strings.Fields(line)
				base := parts[1]
				name := string(stageCount) // Default ID
				
				// Handle "AS alias"
				for i, p := range parts {
					if strings.ToLower(p) == "as" && i+1 < len(parts) {
						name = parts[i+1]
					}
				}

				d.Dockerfile.Stages = append(d.Dockerfile.Stages, BuildStage{
					ID: stageCount, 
					Name: name, 
					BaseImage: base,
				})
				current = &d.Dockerfile.Stages[len(d.Dockerfile.Stages)-1]
			
			} else if current != nil {
				// Detect dependencies between stages
				if strings.HasPrefix(line, "COPY") && strings.Contains(line, "--from=") {
					parts := strings.Fields(line)
					for _, p := range parts {
						if strings.HasPrefix(p, "--from=") {
							src := strings.TrimPrefix(p, "--from=")
							current.Links = append(current.Links, src)
						}
					}
				}
				// Add interesting layers
				if strings.HasPrefix(line, "RUN") || strings.HasPrefix(line, "CMD") || strings.HasPrefix(line, "ENTRYPOINT") {
					// Truncate long lines
					if len(line) > 40 { line = line[:37] + "..." }
					current.Layers = append(current.Layers, line)
				}
			}
		}
		// Mark last stage as final
		if len(d.Dockerfile.Stages) > 0 {
			d.Dockerfile.Stages[len(d.Dockerfile.Stages)-1].IsFinal = true
		}
		file.Close()
	}

	// --- 2. Compose Analysis (Heuristic YAML Parser) ---
	dcPath := filepath.Join(root, "docker-compose.yml")
	if file, err := os.Open(dcPath); err == nil {
		d.Found = true
		d.Compose.Found = true
		
		scanner := bufio.NewScanner(file)
		var svc *Service
		indent := 0
		inServices := false

		for scanner.Scan() {
			raw := scanner.Text()
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") { continue }
			currIndent := len(raw) - len(strings.TrimLeft(raw, " "))

			if line == "services:" { inServices = true; continue }
			if !inServices { continue }

			// New Service Detection (indent 2 usually)
			if currIndent == 2 && strings.HasSuffix(line, ":") {
				name := strings.TrimSuffix(line, ":")
				d.Compose.Services = append(d.Compose.Services, Service{Name: name})
				svc = &d.Compose.Services[len(d.Compose.Services)-1]
				indent = 2
				continue
			}

			// Service Properties
			if svc != nil && currIndent > indent {
				if strings.HasPrefix(line, "image:") {
					svc.Image = strings.TrimPrefix(line, "image: ")
				}
				if strings.HasPrefix(line, "build:") {
					svc.BuildPath = strings.TrimPrefix(line, "build: ")
				}
				// Depends On parsing
				if strings.HasPrefix(line, "-") && (strings.Contains(raw, "depends_on") || svc.Name != "") {
					// This is weak, we assume context. 
					// For v1.4 MVP we rely on "depends_on:" block detection later if needed, 
					// but let's try direct line check if previous line was depends_on
				}
			}
			// Better depends_on check: read full file to memory for context? 
			// Let's stick to simple line scan for MVP v1.4, assuming standard formatting
			if strings.TrimSpace(raw) == "depends_on:" {
				// Next lines are deps
			}
		}
		
		// Re-read for robust depends_on (Multi-pass)
		file.Seek(0, 0)
		scanner = bufio.NewScanner(file)
		var currentSvcName string
		inDepends := false
		
		for scanner.Scan() {
			raw := scanner.Text()
			line := strings.TrimSpace(raw)
			currIndent := len(raw) - len(strings.TrimLeft(raw, " "))
			
			if currIndent == 2 && strings.HasSuffix(line, ":") && line != "services:" {
				currentSvcName = strings.TrimSuffix(line, ":")
				inDepends = false
			}
			
			if line == "depends_on:" {
				inDepends = true
				continue
			}
			
			if inDepends && strings.HasPrefix(line, "-") {
				dep := strings.TrimPrefix(line, "- ")
				// Find service and add dep
				for i := range d.Compose.Services {
					if d.Compose.Services[i].Name == currentSvcName {
						d.Compose.Services[i].Links = append(d.Compose.Services[i].Links, dep)
					}
				}
			} else if inDepends && currIndent <= 4 && line != "" {
				inDepends = false
			}
		}
		
		file.Close()
	}

	return d
}