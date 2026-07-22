package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/effexorxruser/EffexorWinPE/internal/collector"
	"github.com/effexorxruser/EffexorWinPE/internal/reportfile"
)

var version = "dev"

func main() {
	output := flag.String("output", "diagnostic-report.json", "path for the JSON report, or - for stdout")
	pretty := flag.Bool("pretty", true, "indent JSON output")
	flag.Parse()

	report, err := collector.Collect(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect diagnostics: %v\n", err)
		os.Exit(1)
	}

	if err := reportfile.Write(*output, report, *pretty); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}

	if *output != "-" {
		fmt.Printf("Diagnostic report written to %s\n", *output)
	}
}
