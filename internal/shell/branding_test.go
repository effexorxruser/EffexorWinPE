package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
)

func TestNoANPInLocalesAndMockAssets(t *testing.T) {
	for _, locale := range []string{i18n.LocaleRuRU, i18n.LocaleEnUS} {
		cat, err := i18n.Catalog(locale)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range cat {
			if strings.Contains(strings.ToUpper(k+v), "ANP") {
				t.Fatalf("ANP in locale %s: %s=%q", locale, k, v)
			}
		}
	}
	for name, raw := range map[string][]byte{
		"report":    mock.ReportJSON(),
		"diagnosis": mock.DiagnosisJSON(),
		"session":   mock.SessionJSON(),
	} {
		if strings.Contains(strings.ToUpper(string(raw)), "ANP") {
			t.Fatalf("ANP in mock %s", name)
		}
	}
	root := filepath.Join("..", "..", "locales")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToUpper(string(raw)), "ANP") {
			t.Fatalf("ANP in %s", e.Name())
		}
	}
}
