package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/devos-os/d-ci/internal/domain"
)

type GitHubProvider struct {
	token  string
	owner  string
	repo   string
	client *http.Client
	events chan domain.PipelineEvent
	cancel context.CancelFunc
}

func NewGitHubProvider(owner, repo, token string) (*GitHubProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("missing GitHub token")
	}

	return &GitHubProvider{
		token: token,
		owner: owner,
		repo:  repo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		events: make(chan domain.PipelineEvent),
	}, nil
}

func (g *GitHubProvider) Name() string { return "GitHub REST" }

func (g *GitHubProvider) Subscribe() <-chan domain.PipelineEvent {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	go func() {
		defer close(g.events)
		log.Println("⚡ Subscribed via REST API. Starting poll loop...")

		g.poll(ctx) // Первый опрос

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				g.poll(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return g.events
}

// --- Структуры для JSON ответов GitHub REST API ---

type ghRunsResponse struct {
	WorkflowRuns []ghRun `json:"workflow_runs"`
}

type ghRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`     // queued, in_progress, completed
	Conclusion string    `json:"conclusion"` // success, failure, neutral, cancelled, skipped
	HeadBranch string    `json:"head_branch"`
	HeadCommit ghCommit  `json:"head_commit"`
	CreatedAt  time.Time `json:"created_at"`
	HTMLURL    string    `json:"html_url"`
}

type ghCommit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
}

type ghJobsResponse struct {
	Jobs []ghJob `json:"jobs"`
}

type ghJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// --- Логика опроса ---

func (g *GitHubProvider) poll(ctx context.Context) {
	log.Println("📡 Polling GitHub REST API...")

	// 1. Получаем список Workflow Runs
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?per_page=5", g.owner, g.repo)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("❌ API Error: Status %d", resp.StatusCode)
		return
	}

	var runsResp ghRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runsResp); err != nil {
		log.Printf("❌ JSON Error: %v", err)
		return
	}

	log.Printf("✅ Received %d workflows", len(runsResp.WorkflowRuns))

	// 2. Обрабатываем каждый Run
	for _, run := range runsResp.WorkflowRuns {
		p := domain.Pipeline{
			ID:        fmt.Sprintf("%d", run.ID),
			Project:   fmt.Sprintf("%s/%s", g.owner, g.repo),
			Branch:    run.HeadBranch,
			CommitMsg: run.HeadCommit.Message,
			Author:    run.HeadCommit.Author.Name,
			StartedAt: run.CreatedAt,
			Url:       run.HTMLURL,
		}

		// Статус пайплайна
		p.Status = mapStatus(run.Status, run.Conclusion)

		// 3. Получаем Jobs для каждого Run (отдельный запрос)
		// Примечание: В реальном высоконагруженном приложении это нужно кешировать или ограничивать
		jobs, err := g.getJobs(ctx, run.ID)
		if err == nil {
			p.Jobs = jobs
		}

		g.events <- domain.PipelineEvent{Type: "UPDATE", Pipeline: p}
	}
}

func (g *GitHubProvider) getJobs(ctx context.Context, runID int64) ([]domain.Job, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%d/jobs", g.owner, g.repo, runID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jobsResp ghJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobsResp); err != nil {
		return nil, err
	}

	var domainJobs []domain.Job
	for _, j := range jobsResp.Jobs {
		domainJobs = append(domainJobs, domain.Job{
			ID:     fmt.Sprintf("%d", j.ID),
			Name:   j.Name,
			Status: mapStatus(j.Status, j.Conclusion),
		})
	}
	return domainJobs, nil
}

func mapStatus(status, conclusion string) domain.Status {
	if status == "queued" || status == "in_progress" || status == "waiting" {
		return domain.StatusRunning
	}
	if status == "completed" {
		switch conclusion {
		case "success":
			return domain.StatusSuccess
		case "failure", "timed_out", "action_required":
			return domain.StatusFailed
		case "cancelled", "skipped":
			return domain.StatusSkipped
		default:
			return domain.StatusFailed
		}
	}
	return domain.StatusPending
}

func (g *GitHubProvider) Trigger(id string) error { return nil }