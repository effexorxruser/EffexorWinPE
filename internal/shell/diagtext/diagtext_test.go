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

func containsCyrillic(s string) bool {
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			return true
		}
	}
	return false
}
