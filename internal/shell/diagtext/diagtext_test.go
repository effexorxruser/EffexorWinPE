package diagtext

import (
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
)

func TestFindingLocalizationRussian(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	got := FindingTitle(b, "windows.installation-not-found", "No offline Windows installation was detected")
	if got == "No offline Windows installation was detected" {
		t.Fatalf("expected localized title, got fallback %q", got)
	}
	if !containsCyrillic(got) {
		t.Fatalf("expected Cyrillic title, got %q", got)
	}
}

func TestFindingFallbackEnglish(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	fallback := "Totally unknown finding title"
	got := FindingTitle(b, "does.not.exist", fallback)
	if got != fallback {
		t.Fatalf("got %q, want fallback", got)
	}
}

func TestDynamicStorageIDCanonicalization(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	got := FindingTitle(b, "storage.disk.0.health", "Disk reports a non-healthy status")
	if got == "Disk reports a non-healthy status" {
		t.Fatalf("expected localized dynamic storage title, got %q", got)
	}
}

func TestHeadlineBySeverity(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	got := Headline(b, "warning", "Preflight found issues that require additional diagnosis")
	if got == "Preflight found issues that require additional diagnosis" {
		t.Fatalf("expected localized headline, got %q", got)
	}
}

func TestEnumsAndCheckSummaryRussian(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	if !containsCyrillic(Severity(b, "critical")) {
		t.Fatal("severity")
	}
	if !containsCyrillic(Confidence(b, "high")) {
		t.Fatal("confidence")
	}
	if !containsCyrillic(Risk(b, "read_only")) {
		t.Fatal("risk")
	}
	loc, tech := CheckSummary(b, "collector.runtime", "Collector runtime probe completed")
	if !containsCyrillic(loc) || tech == "" {
		t.Fatalf("check summary loc=%q tech=%q", loc, tech)
	}
	lim := Limitation(b, "Offline preflight does not contact the model gateway")
	if !containsCyrillic(lim) {
		t.Fatalf("limitation=%q", lim)
	}
	unknown := Limitation(b, "Some online model prose nobody translated")
	if unknown != "Some online model prose nobody translated" {
		t.Fatalf("unknown limitation must stay raw, got %q", unknown)
	}
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			return true
		}
	}
	return false
}
