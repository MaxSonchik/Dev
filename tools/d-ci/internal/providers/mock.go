package providers

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/devos-os/d-ci/internal/domain"
)

type MockProvider struct {
	updates   chan domain.PipelineEvent
	pipelines []*domain.Pipeline // Храним состояние, чтобы обновлять его
}

func NewMockProvider() *MockProvider {
	// Создаем начальное состояние с Джобами
	ps := []*domain.Pipeline{
		{
			ID: "1", Project: "devos/core", Branch: "main", Author: "Max", CommitMsg: "feat: kernel config",
			Status: domain.StatusRunning,
			Jobs: []domain.Job{
				{ID: "j1", Name: "build-iso", Status: domain.StatusSuccess},
				{ID: "j2", Name: "test-qemu", Status: domain.StatusRunning},
				{ID: "j3", Name: "publish", Status: domain.StatusPending},
			},
		},
		{
			ID: "2", Project: "devos/d-guard", Branch: "feature/audit", Author: "Bot", CommitMsg: "fix: gitleaks rules",
			Status: domain.StatusPending,
			Jobs: []domain.Job{
				{ID: "j4", Name: "lint", Status: domain.StatusPending},
				{ID: "j5", Name: "unit-test", Status: domain.StatusPending},
				{ID: "j6", Name: "security-scan", Status: domain.StatusPending},
			},
		},
		{
			ID: "3", Project: "devos/d-env", Branch: "fix/ui", Author: "Max", CommitMsg: "chore: update deps",
			Status: domain.StatusFailed,
			Jobs: []domain.Job{
				{ID: "j7", Name: "build", Status: domain.StatusSuccess},
				{ID: "j8", Name: "integration", Status: domain.StatusFailed},
				{ID: "j9", Name: "release", Status: domain.StatusSkipped},
			},
		},
	}

	return &MockProvider{
		updates:   make(chan domain.PipelineEvent),
		pipelines: ps,
	}
}

func (m *MockProvider) Name() string { return "Simulator" }

func (m *MockProvider) Subscribe() <-chan domain.PipelineEvent {
	go func() {
		// 1. Отправляем начальное состояние
		for _, p := range m.pipelines {
			m.updates <- domain.PipelineEvent{Type: "NEW", Pipeline: *p}
		}

		// 2. Цикл симуляции прогресса
		for {
			time.Sleep(time.Millisecond * 800) // Обновление каждые 0.8 сек

			// Выбираем случайный пайплайн
			p := m.pipelines[rand.Intn(len(m.pipelines))]

			// Логика сдвига прогресса
			allDone := true
			for i := range p.Jobs {
				j := &p.Jobs[i]
				
				if j.Status == domain.StatusRunning {
					// Завершаем текущий
					if rand.Float32() > 0.1 {
						j.Status = domain.StatusSuccess
					} else {
						j.Status = domain.StatusFailed // Иногда ломаем
						p.Status = domain.StatusFailed
					}
					// Запускаем следующий, если есть
					if i+1 < len(p.Jobs) && p.Status != domain.StatusFailed {
						p.Jobs[i+1].Status = domain.StatusRunning
					}
					allDone = false
					break
				} else if j.Status == domain.StatusPending {
					// Если нашли первый Pending (и предыдущий не бежит), запускаем его
					if i == 0 || p.Jobs[i-1].Status == domain.StatusSuccess {
						j.Status = domain.StatusRunning
						p.Status = domain.StatusRunning
						allDone = false
						break
					}
				} else if j.Status == domain.StatusFailed {
					// Если упал, рестартим через время
					if rand.Float32() > 0.8 {
						resetPipeline(p)
					}
					allDone = false
					break
				}
			}

			if allDone {
				// Если все зеленые, рестартим для демонстрации
				if rand.Float32() > 0.9 {
					resetPipeline(p)
				}
			}

			// Отправка обновления
			m.updates <- domain.PipelineEvent{Type: "UPDATE", Pipeline: *p}
		}
	}()
	return m.updates
}

func (m *MockProvider) Trigger(id string) error { return nil }

func resetPipeline(p *domain.Pipeline) {
	p.Status = domain.StatusPending
	for i := range p.Jobs {
		p.Jobs[i].Status = domain.StatusPending
	}
	p.CommitMsg = fmt.Sprintf("update: iteration %d", rand.Intn(999))
}