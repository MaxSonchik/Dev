package core

import "fmt"

// Severity levels
type Severity string

const (
	SevCritical Severity = "CRITICAL"
	SevHigh     Severity = "HIGH"
	SevMedium   Severity = "MEDIUM"
	SevLow      Severity = "LOW"
)

// Issue представляет одну найденную проблему
type Issue struct {
	Scanner     string   // Имя сканера (e.g., "Secrets", "Docker")
	Severity    Severity
	Message     string
	File        string
	Line        int
	Description string   // Подробное описание или ссылка на CVE
	Suggestion  string   // Как исправить
}

func (i Issue) String() string {
	return fmt.Sprintf("[%s][%s] %s (%s:%d)", i.Scanner, i.Severity, i.Message, i.File, i.Line)
}

// Config конфигурация запуска
type Config struct {
	IsCI        bool     // Режим CI/CD (без UI, strict exit codes)
	ScanAll     bool     // Сканировать всё или только изменения?
	BaseBranch  string   // С чем сравнивать (обычно main или master)
	OutputFmt   string   // json, sarif, table
}