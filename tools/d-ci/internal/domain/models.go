package domain

import "time"

type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusPending Status = "pending"
	StatusSkipped Status = "skipped"
)

// Pipeline - абстрактное представление CI пайплайна
type Pipeline struct {
	ID        string
	Project   string    // Название репо (owner/repo)
	Branch    string
	CommitMsg string
	Author    string
	Status    Status
	StartedAt time.Time
	Duration  time.Duration
	Url       string
	Jobs      []Job     // Список джобов внутри пайплайна
}

type Job struct {
	ID     string
	Name   string
	Status Status
}

// Provider - интерфейс, который должны реализовать GitHub/GitLab адаптеры
type Provider interface {
	Name() string
	// Subscribe возвращает канал, в который будут лететь обновления в реальном времени
	Subscribe() <-chan PipelineEvent 
	// Trigger перезапускает пайплайн
	Trigger(pipelineID string) error
}

// PipelineEvent - событие изменения (для UI)
type PipelineEvent struct {
	Type     string // "UPDATE", "NEW", "DELETE"
	Pipeline Pipeline
}