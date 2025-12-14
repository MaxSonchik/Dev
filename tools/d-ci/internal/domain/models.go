package domain

import "time"

type Status string

const (
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusPending  Status = "pending"
	StatusSkipped  Status = "skipped"
	StatusCanceled Status = "canceled"
	StatusManual   Status = "manual" // Важно для GitLab
)

// Pipeline - корневая сущность
type Pipeline struct {
	ID        string
	Project   string // "owner/repo"
	Ref       string // branch or tag
	CommitMsg string
	Author    string
	Status    Status
	CreatedAt time.Time
	Duration  time.Duration
	WebURL    string
	
	// GitLab имеет Stages, GitHub - плоский список (но мы можем сгруппировать)
	Stages []Stage 
}

// Stage - группа джобов (например: "build", "test", "deploy")
type Stage struct {
	Name   string
	Status Status
	Jobs   []Job
}

type Job struct {
	ID          string
	Name        string
	Status      Status
	StartedAt   time.Time
	Duration    time.Duration
	WebURL      string
	AllowFailure bool
}

// Provider - расширенный интерфейс
type Provider interface {
	// Инициализация (проверка доступа)
	Ping() error
	
	// Основной цикл получения данных
	Subscribe() <-chan PipelineEvent
	
	// Actions
	RetryPipeline(pid string) error
	CancelPipeline(pid string) error
	RetryJob(jobId string) error
	
	// Logs
	GetJobLog(jobId string) (string, error)
}

type PipelineEvent struct {
	RepoName string // Чтобы UI знал, к какому репо относится обновление
	Type     string // "UPDATE", "ERROR"
	Pipeline Pipeline
	Error    error
}