package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/devos-os/d-recon/internal/core"
	"github.com/devos-os/d-recon/internal/engines"
	"github.com/devos-os/d-recon/internal/ui"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	// Flags
	flagNmap       bool
	flagMasscan    bool
	flagBbot       bool
	flagCrt        bool
	flagSherlock   string // Username to hunt
	flagLoki       string // Path to scan
	flagAggressive bool   // All-in mode
	flagJson       bool
	
	// Engine Args
	nmapArgs string
)

func main() {
	godotenv.Load(".env")

	var rootCmd = &cobra.Command{
		Use:   "d-recon [target]",
		Short: "DevOS Recon Orchestrator v2.0",
		Long:  "Aggregates Nmap, Masscan, BBOT, Sherlock, Loki and CRT.sh.",
		// Args:  cobra.ExactArgs(1), // Убрали ExactArgs, так как для Sherlock цель - это юзернейм
		Run:   run,
	}

	// Network Recon
	rootCmd.Flags().BoolVar(&flagNmap, "nmap", false, "Active Nmap Scan")
	rootCmd.Flags().StringVar(&nmapArgs, "nmap-args", "-sV -O -T4", "Custom Nmap flags") // Дефолт здесь
	rootCmd.Flags().BoolVar(&flagMasscan, "masscan", false, "Active Masscan (requires sudo)")
	
	// OSINT
	rootCmd.Flags().BoolVar(&flagBbot, "bbot", false, "Run BBOT OSINT")
	rootCmd.Flags().BoolVar(&flagCrt, "crt", false, "Passive Domain Search (CRT.sh)")
	rootCmd.Flags().StringVar(&flagSherlock, "sherlock", "", "Hunt username (Identity Recon)")
	
	// Threat Intel
	rootCmd.Flags().StringVar(&flagLoki, "loki", "", "Scan path for IOCs (Malware Recon)")

	// Meta
	rootCmd.Flags().BoolVar(&flagAggressive, "aggressive", false, "Enable ALL applicable scanners")
	rootCmd.Flags().BoolVar(&flagJson, "json", false, "Output JSON")

	if err := rootCmd.Execute(); err != nil { os.Exit(1) }
}

func run(cmd *cobra.Command, args []string) {
	var target string
	if len(args) > 0 {
		target = args[0]
	}

	// Auto-enable aggressive mode
	if flagAggressive {
		flagNmap = true
		flagCrt = true
		// Masscan и BBOT тяжелые, их лучше включать явно или если пользователь уверен
		// Но для aggressive добавим Nmap -A
		if nmapArgs == "-sV -O -T4" {
			nmapArgs = "-A -T4" // Aggressive Nmap
		}
	}

	var results []core.Host

	// 1. Identity Recon (Sherlock) - не требует IP
	if flagSherlock != "" {
		res, err := engines.RunSherlock(flagSherlock)
		if err == nil { results = merge(results, res) }
	}

	// 2. Malware Recon (Loki) - не требует IP
	if flagLoki != "" {
		res, err := engines.RunLoki(flagLoki)
		if err == nil { results = merge(results, res) }
	}

	// Остальным нужен Target IP/Domain
	if target != "" {
		// Passive
		if flagCrt {
			res, _ := engines.RunCrtSh(target)
			results = merge(results, res)
		}
		if flagBbot {
			res, _ := engines.RunBBOT(target)
			results = merge(results, res)
		}

		// Active
		if flagMasscan {
			res, err := engines.RunMasscan(target, "0-1000") // Default ports for auto
			if err != nil && !flagJson { fmt.Printf("⚠️ Masscan: %v\n", err) }
			results = merge(results, res)
		}

		if flagNmap {
			if !flagJson { fmt.Printf("🚀 Nmap args: %s\n", nmapArgs) }
			res, err := engines.RunNmap(target, nmapArgs)
			if err != nil && !flagJson { fmt.Printf("⚠️ Nmap: %v\n", err) }
			results = merge(results, res)
		}
	}

	// Output
	if flagJson {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	} else {
		if len(results) == 0 {
			fmt.Println("No results found. Try --nmap or --crt")
		} else {
			ui.PrintResults(results)
		}
	}
}

func merge(base []core.Host, new []core.Host) []core.Host {
	return append(base, new...)
}