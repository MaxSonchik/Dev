package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		log.Printf("⚡ [GitHub] Subscribed to %s/%s", g.owner, g.repo)

		// Первичный опрос
		g.poll(ctx)

		// Цикл опроса (GitHub API rate limit - осторожно, ставим 10 сек)
		ticker := time.NewTicker(10 * time.Second)
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

// --- API Polling Logic ---

type ghRunsResponse struct {
	WorkflowRuns []ghRun `json:"workflow_runs"`
}

type ghRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`     // queued, in_progress, completed
	Conclusion string    `json:"conclusion"` // success, failure, cancelled...
	HeadBranch string    `json:"head_branch"`
	HeadCommit ghCommit  `json:"head_commit"`
	CreatedAt  time.Time `json:"created_at"`
	HTMLURL    string    `json:"html_url"`
}

type ghCommit struct {
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
	HTMLURL    string `json:"html_url"`
}

func (g *GitHubProvider) poll(ctx context.Context) {
	// 1. Получаем список пайплайнов
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?per_page=5", g.owner, g.repo)
	body, err := g.doRequest("GET", url)
	if err != nil {
		g.sendError(err)
		return
	}

	var runsResp ghRunsResponse
	if err := json.Unmarshal(body, &runsResp); err != nil {
		g.sendError(fmt.Errorf("json parse error: %v", err))
		return
	}

	// 2. Обрабатываем каждый пайплайн
	for _, run := range runsResp.WorkflowRuns {
		pipeline := domain.Pipeline{
			ID:        fmt.Sprintf("%d", run.ID),
			Project:   fmt.Sprintf("%s/%s", g.owner, g.repo),
			Ref:       run.HeadBranch,
			CommitMsg: run.HeadCommit.Message,
			Author:    run.HeadCommit.Author.Name,
			Status:    mapGhStatus(run.Status, run.Conclusion),
			CreatedAt: run.CreatedAt,
			WebURL:    run.HTMLURL,
		}

		// 3. Получаем джобы для этого пайплайна
		jobs, err := g.getJobs(run.ID)
		if err == nil {
			// GitHub не имеет явных "Stages", поэтому группируем всё в одну стадию
			pipeline.Stages = []domain.Stage{
				{
					Name:   "Workflow",
					Status: pipeline.Status,
					Jobs:   jobs,
				},
			}
		}

		// Отправляем обновление в UI
		g.events <- domain.PipelineEvent{
			RepoName: pipeline.Project,
			Type:     "UPDATE",
			Pipeline: pipeline,
		}
	}
}

func (g *GitHubProvider) getJobs(runID int64) ([]domain.Job, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%d/jobs", g.owner, g.repo, runID)
	body, err := g.doRequest("GET", url)
	if err != nil {
		return nil, err
	}

	var jobsResp ghJobsResponse
	if err := json.Unmarshal(body, &jobsResp); err != nil {
		return nil, err
	}

	var domainJobs []domain.Job
	for _, j := range jobsResp.Jobs {
		domainJobs = append(domainJobs, domain.Job{
			ID:      fmt.Sprintf("%d", j.ID),
			Name:    j.Name,
			Status:  mapGhStatus(j.Status, j.Conclusion),
			WebURL:  j.HTMLURL,
		})
	}
	return domainJobs, nil
}

// --- Actions Implementation ---

func (g *GitHubProvider) Ping() error {
	_, err := g.doRequest("GET", "https://api.github.com/user")
	return err
}

func (g *GitHubProvider) RetryPipeline(pid string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s/rerun", g.owner, g.repo, pid)
	_, err := g.doRequest("POST", url)
	return err
}

func (g *GitHubProvider) CancelPipeline(pid string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s/cancel", g.owner, g.repo, pid)
	_, err := g.doRequest("POST", url)
	return err
}

func (g *GitHubProvider) RetryJob(jobId string) error {
	// GitHub API не позволяет перезапустить конкретный job по его ID через публичное API просто так,
	// обычно перезапускают failed jobs через run endpoint.
	// Для MVP вернем ошибку с подсказкой.
	return fmt.Errorf("GitHub API requires full run retry. Press 'r' on Pipeline.")
}

func (g *GitHubProvider) GetJobLog(jobId string) (string, error) {
	// GitHub API возвращает 302 Redirect на raw log. http.Client следует за редиректом автоматически.
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/jobs/%s/logs", g.owner, g.repo, jobId)
	body, err := g.doRequest("GET", url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// --- Helpers ---

func (g *GitHubProvider) doRequest(method, url string) ([]byte, error) {
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// GitHub возвращает 201 Created при успешном перезапуске, 204 при отмене
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (g *GitHubProvider) sendError(err error) {
	g.events <- domain.PipelineEvent{
		RepoName: fmt.Sprintf("%s/%s", g.owner, g.repo),
		Type:     "ERROR",
		Error:    err,
	}
}

func mapGhStatus(status, conclusion string) domain.Status {
	if status == "queued" || status == "in_progress" || status == "waiting" {
		return domain.StatusRunning
	}
	if status == "completed" {
		switch conclusion {
		case "success":
			return domain.StatusSuccess
		case "failure", "timed_out", "action_required":
			return domain.StatusFailed
		case "cancelled":
			return domain.StatusCanceled
		case "skipped":
			return domain.StatusSkipped
		default:
			return domain.StatusFailed
		}
	}
	return domain.StatusPending
}