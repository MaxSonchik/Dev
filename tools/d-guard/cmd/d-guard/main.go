package main

import (
	"fmt"
	"os"

	"github.com/devos-os/d-guard/internal"
	"github.com/devos-os/d-guard/internal/core"
	"github.com/devos-os/d-guard/internal/reporters"
	"github.com/spf13/cobra"
)

var cfg core.Config
var reportFile string
var strictMode bool // <--- Новый флаг

func main() {
	var rootCmd = &cobra.Command{
		Use:   "d-guard",
		Short: "DevOS Security Orchestrator",
		Run:   run,
	}
	rootCmd.PersistentFlags().BoolVar(&cfg.IsCI, "ci", false, "CI Mode (Diff vs Base Branch)")
	rootCmd.PersistentFlags().BoolVar(&cfg.ScanAll, "all", false, "Scan All Files")
	rootCmd.PersistentFlags().StringVar(&cfg.BaseBranch, "base", "", "Diff Base")
	rootCmd.PersistentFlags().StringVar(&reportFile, "report", "", "HTML Report file")
	
	// Добавляем флаг strict
	rootCmd.PersistentFlags().BoolVar(&strictMode, "strict", false, "Exit with code 1 if issues found (for pre-commit)")

	if err := rootCmd.Execute(); err != nil { os.Exit(1) }
}

func run(cmd *cobra.Command, args []string) {
	issues := internal.RunAll(cfg)

	if reportFile != "" {
		reporters.GenerateHTML(issues, reportFile)
	}
	
	if len(issues) > 0 {
		fmt.Printf("\n🔥 Total Issues: %d\n", len(issues))
		for _, i := range issues {
			color := "\033[33m" // Yellow
			if i.Severity == core.SevCritical { color = "\033[31m" }
			reset := "\033[0m"
			fmt.Printf("%s[%s] %s%s: %s\n    %s\n", color, i.Severity, i.Scanner, reset, i.Message, i.File)
		}
		
		// Ломаем процесс, если включен CI ИЛИ Strict
		if cfg.IsCI || strictMode {
			os.Exit(1)
		}
	} else {
		fmt.Println("\n✨ All clear. Good job.")
	}
}