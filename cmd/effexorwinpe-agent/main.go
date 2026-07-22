package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/triage"
)

var version = "dev"

func main() {
	input := flag.String("input", "diagnostic-report.json", "path to a diagnostic report, or - for stdin")
	output := flag.String("output", "diagnosis.json", "path for the assessment, or - for stdout")
	pretty := flag.Bool("pretty", true, "indent JSON output")
	flag.Parse()

	report, err := readReport(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read diagnostic report: %v\n", err)
		os.Exit(1)
	}
	assessment, err := triage.Analyze(report, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze diagnostic report: %v\n", err)
		os.Exit(1)
	}
	if err := writeJSON(*output, assessment, *pretty); err != nil {
		fmt.Fprintf(os.Stderr, "write diagnosis: %v\n", err)
		os.Exit(1)
	}
	if *output != "-" {
		fmt.Printf("EffexorWinPE agent %s wrote offline preflight to %s\n", version, *output)
	}
}

func readReport(path string) (diagnostics.Report, error) {
	var reader io.Reader
	if path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return diagnostics.Report{}, err
		}
		defer file.Close()
		reader = file
	}
	var report diagnostics.Report
	if err := json.NewDecoder(reader).Decode(&report); err != nil {
		return diagnostics.Report{}, err
	}
	return report, nil
}

func writeJSON(path string, value any, pretty bool) error {
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(value, "", "  ")
	} else {
		data, err = json.Marshal(value)
	}
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}
