package providers

import (
	"fmt"
	//"math/rand"
	"time"

	"github.com/devos-os/d-ci/internal/domain"
)

type MockProvider struct {
	repoName  string
	updates   chan domain.PipelineEvent
	pipelines []*domain.Pipeline
}

func NewMockProvider(repoName string) *MockProvider {
	if repoName == "" {
		repoName = "devos/core"
	}
	
	// Генерируем тестовые данные с Stages
	ps := []*domain.Pipeline{
		createMockPipeline("1", repoName, "main", "feat: initial commit", domain.StatusRunning),
		createMockPipeline("2", repoName, "dev", "fix: logic error", domain.StatusFailed),
	}

	return &MockProvider{
		repoName:  repoName,
		updates:   make(chan domain.PipelineEvent),
		pipelines: ps,
	}
}

func createMockPipeline(id, repo, branch, msg string, status domain.Status) *domain.Pipeline {
	return &domain.Pipeline{
		ID: id, Project: repo, Ref: branch, CommitMsg: msg, Author: "MockUser",
		Status: status, CreatedAt: time.Now(), WebURL: "https://localhost",
		Stages: []domain.Stage{
			{
				Name: "build", Status: domain.StatusSuccess,
				Jobs: []domain.Job{{ID: id + "j1", Name: "go-build", Status: domain.StatusSuccess}},
			},
			{
				Name: "test", Status: status,
				Jobs: []domain.Job{
					{ID: id + "j2", Name: "unit-test", Status: domain.StatusSuccess},
					{ID: id + "j3", Name: "integration", Status: status},
				},
			},
		},
	}
}

func (m *MockProvider) Name() string { return "Mock: " + m.repoName }

func (m *MockProvider) Ping() error { return nil }

func (m *MockProvider) Subscribe() <-chan domain.PipelineEvent {
	go func() {
		// Начальная отправка
		for _, p := range m.pipelines {
			m.updates <- domain.PipelineEvent{RepoName: m.repoName, Type: "NEW", Pipeline: *p}
		}
		// Имитация активности
		for {
			time.Sleep(2 * time.Second)
			p := m.pipelines[0] // Меняем только первый для теста
			if p.Status == domain.StatusRunning {
				p.Status = domain.StatusSuccess
				p.Stages[1].Status = domain.StatusSuccess
				p.Stages[1].Jobs[1].Status = domain.StatusSuccess
			} else {
				p.Status = domain.StatusRunning
				p.Stages[1].Status = domain.StatusRunning
				p.Stages[1].Jobs[1].Status = domain.StatusRunning
			}
			m.updates <- domain.PipelineEvent{RepoName: m.repoName, Type: "UPDATE", Pipeline: *p}
		}
	}()
	return m.updates
}

func (m *MockProvider) RetryPipeline(pid string) error { return nil }
func (m *MockProvider) CancelPipeline(pid string) error { return nil }
func (m *MockProvider) RetryJob(jobId string) error { return nil }
func (m *MockProvider) GetJobLog(jobId string) (string, error) {
	return fmt.Sprintf("Mock Logs for Job %s\nRunning...\nDone.", jobId), nil
}