package container

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devos-os/d-guard/internal/core"
	"github.com/docker/docker/api/types" // <--- ИЗМЕНЕНИЕ: Используем общий types
	"github.com/docker/docker/client"
)

// Scan запускает и статический, и динамический анализ
func Scan(files []string) []core.Issue {
	var issues []core.Issue

	// 1. Static Analysis (Dockerfile)
	for _, path := range files {
		if strings.HasSuffix(path, "Dockerfile") {
			issues = append(issues, scanDockerfile(path)...)
		}
	}

	// 2. Runtime Analysis (Docker Daemon)
	// Используем WithAPIVersionNegotiation для совместимости
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err == nil {
		defer cli.Close()
		runtimeIssues := scanRuntime(cli)
		issues = append(issues, runtimeIssues...)
	}

	return issues
}

// --- STATIC ANALYSIS ---
func scanDockerfile(path string) []core.Issue {
	var issues []core.Issue
	f, err := os.Open(path)
	if err != nil { return nil }
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	hasUser := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "FROM") && strings.HasSuffix(line, ":latest") {
			issues = append(issues, core.Issue{
				Scanner: "Docker Static", Severity: core.SevMedium, File: path, Line: lineNum,
				Message: "Base image uses ':latest' tag",
				Suggestion: "Pin specific version (e.g., node:18-alpine) for reproducibility",
			})
		}

		if strings.HasPrefix(line, "ADD") {
			issues = append(issues, core.Issue{
				Scanner: "Docker Static", Severity: core.SevLow, File: path, Line: lineNum,
				Message: "Use 'COPY' instead of 'ADD'",
				Suggestion: "'ADD' can fetch remote URLs and unpack archives unexpectedly",
			})
		}

		if strings.HasPrefix(line, "ENV") && (strings.Contains(line, "KEY") || strings.Contains(line, "SECRET") || strings.Contains(line, "PASSWORD")) {
			issues = append(issues, core.Issue{
				Scanner: "Docker Static", Severity: core.SevHigh, File: path, Line: lineNum,
				Message: "Potential secret in ENV variable",
				Suggestion: "Use Build Args or mount secrets at runtime",
			})
		}

		if strings.HasPrefix(line, "USER") { hasUser = true }
	}

	if !hasUser {
		issues = append(issues, core.Issue{
			Scanner: "Docker Static", Severity: core.SevMedium, File: path, Line: 1,
			Message: "Running as root (No USER instruction)",
			Suggestion: "Create a non-root user and switch to it using 'USER'",
		})
	}

	return issues
}

// --- RUNTIME ANALYSIS ---
func scanRuntime(cli *client.Client) []core.Issue {
	var issues []core.Issue
	
	// ИЗМЕНЕНИЕ: types.ContainerListOptions вместо container.ListOptions
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil { return nil }

	for _, c := range containers {
		info, err := cli.ContainerInspect(context.Background(), c.ID)
		if err != nil { continue }

		name := strings.TrimPrefix(info.Name, "/")

		if info.HostConfig.Privileged {
			issues = append(issues, core.Issue{
				Scanner: "Docker Runtime", Severity: core.SevCritical,
				Message: fmt.Sprintf("Container '%s' is running in PRIVILEGED mode", name),
				Suggestion: "Disable privileged mode unless absolutely necessary",
			})
		}

		for port := range info.NetworkSettings.Ports {
			if port.Port() == "22" {
				issues = append(issues, core.Issue{
					Scanner: "Docker Runtime", Severity: core.SevHigh,
					Message: fmt.Sprintf("Container '%s' exposes SSH port 22", name),
					Suggestion: "Do not run SSH inside containers",
				})
			}
		}

		for _, mount := range info.Mounts {
			if strings.Contains(mount.Source, "docker.sock") {
				issues = append(issues, core.Issue{
					Scanner: "Docker Runtime", Severity: core.SevHigh,
					Message: fmt.Sprintf("Container '%s' has docker.sock mounted", name),
					Suggestion: "This gives full root access to the host. Use API proxy instead.",
				})
			}
		}
	}

	return issues
}