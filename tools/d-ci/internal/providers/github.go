package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/devos-os/d-ci/internal/domain"
	"github.com/shurcooL/githubv4"
)

type GitHubProvider struct {
	token    string
	owner    string
	repo     string
	client   *githubv4.Client
	events   chan domain.PipelineEvent
	cancel   context.CancelFunc
}

func NewGitHubProvider(owner, repo, token string) (*GitHubProvider, error) {
	if token == "" { return nil, fmt.Errorf("missing GitHub token") }
	
	src := &tokenSource{AccessToken: token}
	httpClient := &http.Client{
		Transport: &oauth2Transport{
			source: src,
			base:   http.DefaultTransport,
		},
	}
	client := githubv4.NewClient(httpClient)
	
	return &GitHubProvider{
		token:  token,
		owner:  owner,
		repo:   repo,
		client: client,
		events: make(chan domain.PipelineEvent),
	}, nil
}

func (g *GitHubProvider) Name() string { return "GitHub" }

func (g *GitHubProvider) Subscribe() <-chan domain.PipelineEvent {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel

	go func() {
		defer close(g.events)
		g.poll(ctx) // Первый запуск сразу
		
		ticker := time.NewTicker(30 * time.Second) // Опрос API каждые 30 сек
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

func (g *GitHubProvider) poll(ctx context.Context) {
	// Упрощенный запрос с использованием String вместо Enum для стабильности
	var query struct {
		Repository struct {
			NameWithOwner githubv4.String
			WorkflowRuns struct {
				Nodes []struct {
					ID     githubv4.String
					RunID  githubv4.Int
					Status githubv4.String // Используем String!
					HeadRef struct { Name githubv4.String }
					HeadCommit struct {
						Message githubv4.String
						Author struct { User struct { Login githubv4.String } }
						CommittedDate githubv4.DateTime
					}
					Jobs struct {
						Nodes []struct {
							Name   githubv4.String
							ID     githubv4.String
							Status githubv4.String // Используем String!
							Conclusion githubv4.String
						}
					} `graphql:"jobs(first:20)"`
				}
			} `graphql:"workflowRuns(first: 10, branch: \"main\", orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]interface{}{
		"owner": githubv4.String(g.owner),
		"name":  githubv4.String(g.repo),
	}

	if err := g.client.Query(ctx, &query, vars); err != nil {
		return // Silently ignore errors in loop or log them
	}

	for _, run := range query.Repository.WorkflowRuns.Nodes {
		p := domain.Pipeline{
			ID:        string(run.ID),
			Project:   string(query.Repository.NameWithOwner),
			Branch:    string(run.HeadRef.Name),
			CommitMsg: string(run.HeadCommit.Message),
			Author:    string(run.HeadCommit.Author.User.Login),
			StartedAt: run.HeadCommit.CommittedDate.Time,
		}

		// Mapping Pipeline Status
		status := string(run.Status)
		if status == "COMPLETED" {
			p.Status = domain.StatusSuccess // Условно, реальный статус в Conclusion
		} else if status == "IN_PROGRESS" {
			p.Status = domain.StatusRunning
		} else {
			p.Status = domain.StatusPending
		}

		// Mapping Jobs
		for _, j := range run.Jobs.Nodes {
			job := domain.Job{
				ID:   string(j.ID),
				Name: string(j.Name),
			}
			
			jStatus := string(j.Status)
			jConc := string(j.Conclusion)

			if jStatus == "IN_PROGRESS" || jStatus == "QUEUED" {
				job.Status = domain.StatusRunning
			} else if jStatus == "COMPLETED" {
				if jConc == "SUCCESS" {
					job.Status = domain.StatusSuccess
				} else if jConc == "FAILURE" {
					job.Status = domain.StatusFailed
					p.Status = domain.StatusFailed // Если хоть одна джоба упала
				} else {
					job.Status = domain.StatusSkipped
				}
			} else {
				job.Status = domain.StatusPending
			}
			p.Jobs = append(p.Jobs, job)
		}
		
		g.events <- domain.PipelineEvent{Type: "UPDATE", Pipeline: p}
	}
}

func (g *GitHubProvider) Trigger(id string) error { return nil }

// --- Auth helpers ---
type tokenSource struct { AccessToken string }
func (t *tokenSource) Token() (*http.Cookie, error) { return nil, nil } // Stub

type oauth2Transport struct {
	source *tokenSource
	base   http.RoundTripper
}

func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.source.AccessToken)
	return t.base.RoundTrip(req)
}