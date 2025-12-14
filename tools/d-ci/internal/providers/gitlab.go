package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/devos-os/d-ci/internal/domain"
)

type GitLabProvider struct {
	baseURL    string
	token      string
	projectID  string // URL-encoded path (group/project)
	repoName   string // Human readable name
	client     *http.Client
	events     chan domain.PipelineEvent
}

func NewGitLabProvider(baseURL, token, projectPath string) *GitLabProvider {
	// ID проекта в URL должен быть закодирован (slash -> %2F)
	encodedID := url.PathEscape(projectPath)
	
	return &GitLabProvider{
		baseURL:   baseURL,
		token:     token,
		projectID: encodedID,
		repoName:  projectPath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		events: make(chan domain.PipelineEvent),
	}
}

func (g *GitLabProvider) Name() string { return "GitLab API" }

func (g *GitLabProvider) Subscribe() <-chan domain.PipelineEvent {
	go func() {
		defer close(g.events)
		log.Printf("⚡ [GitLab] Subscribed to %s", g.repoName)

		// Бесконечный цикл опроса
		for {
			g.poll()
			time.Sleep(5 * time.Second) // Пауза между опросами
		}
	}()
	return g.events
}

// --- API Polling Logic ---

type glPipeline struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	Ref       string    `json:"ref"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
}

type glJob struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Stage        string    `json:"stage"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	Duration     float64   `json:"duration"`
	WebURL       string    `json:"web_url"`
	AllowFailure bool      `json:"allow_failure"`
}

func (g *GitLabProvider) poll() {
	// 1. Получаем пайплайны
	url := fmt.Sprintf("%s/api/v4/projects/%s/pipelines?per_page=5", g.baseURL, g.projectID)
	body, err := g.doRequest("GET", url)
	if err != nil {
		g.sendError(err)
		return
	}

	var glPipelines []glPipeline
	if err := json.Unmarshal(body, &glPipelines); err != nil {
		g.sendError(err)
		return
	}

	// 2. Детализация каждого пайплайна
	for _, gp := range glPipelines {
		// Получаем джобы для группировки в стейджи
		jobs, err := g.getJobs(gp.ID)
		if err != nil {
			log.Printf("GitLab job fetch failed: %v", err)
			continue // Пропускаем этот пайплайн, если не смогли получить джобы
		}

		pipeline := domain.Pipeline{
			ID:        fmt.Sprintf("%d", gp.ID),
			Project:   g.repoName,
			Ref:       gp.Ref,
			Status:    mapGlStatus(gp.Status),
			CreatedAt: gp.CreatedAt,
			WebURL:    gp.WebURL,
			// Группируем джобы по стадиям
			Stages:    groupJobsIntoStages(jobs),
		}

		g.events <- domain.PipelineEvent{
			RepoName: g.repoName,
			Type:     "UPDATE",
			Pipeline: pipeline,
		}
	}
}

func (g *GitLabProvider) getJobs(pipelineID int) ([]glJob, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%d/jobs", g.baseURL, g.projectID, pipelineID)
	body, err := g.doRequest("GET", url)
	if err != nil {
		return nil, err
	}
	var jobs []glJob
	if err := json.Unmarshal(body, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// Логика превращения списка джобов в красивые стадии
func groupJobsIntoStages(jobs []glJob) []domain.Stage {
	// Сохраняем порядок появления стадий
	var stagesOrder []string
	stagesMap := make(map[string][]domain.Job)
	
	for _, j := range jobs {
		if _, exists := stagesMap[j.Stage]; !exists {
			stagesOrder = append(stagesOrder, j.Stage)
		}
		
		dJob := domain.Job{
			ID:           fmt.Sprintf("%d", j.ID),
			Name:         j.Name,
			Status:       mapGlStatus(j.Status),
			StartedAt:    j.StartedAt,
			Duration:     time.Duration(j.Duration) * time.Second,
			WebURL:       j.WebURL,
			AllowFailure: j.AllowFailure,
		}
		stagesMap[j.Stage] = append(stagesMap[j.Stage], dJob)
	}

	var result []domain.Stage
	for _, stageName := range stagesOrder {
		// Определяем общий статус стадии
		stageJobs := stagesMap[stageName]
		stageStatus := domain.StatusSuccess
		
		hasRunning := false
		hasFailed := false
		
		for _, j := range stageJobs {
			if j.Status == domain.StatusRunning || j.Status == domain.StatusPending {
				hasRunning = true
			}
			if j.Status == domain.StatusFailed && !j.AllowFailure {
				hasFailed = true
			}
		}
		
		if hasFailed {
			stageStatus = domain.StatusFailed
		} else if hasRunning {
			stageStatus = domain.StatusRunning
		}

		result = append(result, domain.Stage{
			Name:   stageName,
			Status: stageStatus,
			Jobs:   stageJobs,
		})
	}
	
	return result
}

// --- Actions Implementation ---

func (g *GitLabProvider) Ping() error {
	url := fmt.Sprintf("%s/api/v4/projects/%s", g.baseURL, g.projectID)
	_, err := g.doRequest("GET", url)
	return err
}

func (g *GitLabProvider) RetryPipeline(pid string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/retry", g.baseURL, g.projectID, pid)
	_, err := g.doRequest("POST", url)
	return err
}

func (g *GitLabProvider) CancelPipeline(pid string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/cancel", g.baseURL, g.projectID, pid)
	_, err := g.doRequest("POST", url)
	return err
}

func (g *GitLabProvider) RetryJob(jobId string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/retry", g.baseURL, g.projectID, jobId)
	_, err := g.doRequest("POST", url)
	return err
}

func (g *GitLabProvider) GetJobLog(jobId string) (string, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace", g.baseURL, g.projectID, jobId)
	body, err := g.doRequest("GET", url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// --- Helpers ---

func (g *GitLabProvider) doRequest(method, url string) ([]byte, error) {
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("PRIVATE-TOKEN", g.token)
	
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

func (g *GitLabProvider) sendError(err error) {
	g.events <- domain.PipelineEvent{
		RepoName: g.repoName,
		Type:     "ERROR",
		Error:    err,
	}
}

func mapGlStatus(s string) domain.Status {
	switch s {
	case "success": return domain.StatusSuccess
	case "failed": return domain.StatusFailed
	case "running": return domain.StatusRunning
	case "pending", "created": return domain.StatusPending
	case "skipped": return domain.StatusSkipped
	case "canceled": return domain.StatusCanceled
	case "manual": return domain.StatusManual
	default: return domain.StatusPending
	}
}