package reportfile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func Write(path string, report diagnostics.Report, pretty bool) error {
	if path == "-" {
		return encode(os.Stdout, report, pretty)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".anp-report-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := encode(temporary, report, pretty); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary report: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace report: %w", err)
	}
	return nil
}

func encode(writer io.Writer, report diagnostics.Report, pretty bool) error {
	encoder := json.NewEncoder(writer)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}
