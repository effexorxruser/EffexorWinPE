package present

import (
	"strings"
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
)

func TestRenderMockScreensContainRussianLabels(t *testing.T) {
	b, err := i18n.New(i18n.LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	model, err := mock.AppModel()
	if err != nil {
		t.Fatal(err)
	}
	for _, screen := range NavItems() {
		text := Render(b, screen, model)
		if text == "" {
			t.Fatalf("empty render for %s", screen)
		}
		if strings.Contains(strings.ToUpper(text), "ANP") {
			t.Fatalf("ANP leaked into screen %s", screen)
		}
	}
	overview := Render(b, ScreenOverview, model)
	if !strings.Contains(overview, "EffexorWinPE") {
		t.Fatal("brand missing from overview")
	}
	if !strings.Contains(overview, "Диагностика") && !strings.Contains(overview, b.T("label.diagnostics")) {
		t.Fatalf("diagnostics label missing: %s", overview)
	}
	for _, needle := range []string{b.T("label.checks_ok"), b.T("label.checks_warning"), b.T("label.checks_error"), b.T("label.checks_unknown")} {
		if !strings.Contains(overview, needle) {
			t.Fatalf("overview missing status counter %q:\n%s", needle, overview)
		}
	}
	summary := Render(b, ScreenSummary, model)
	human, _, _ := strings.Cut(summary, b.T("msg.technical_details"))
	if strings.Contains(human, "Collector runtime") {
		t.Fatalf("raw English check summary leaked into human summary:\n%s", summary)
	}
	if !strings.Contains(summary, b.T("msg.technical_details")) {
		t.Fatalf("expected technical details section:\n%s", summary)
	}
	if !strings.Contains(summary, "Среда выполнения collector проверена") {
		t.Fatalf("expected localized check summary:\n%s", summary)
	}
	agent := Render(b, ScreenAgent, model)
	if !strings.Contains(agent, b.T("severity.warning")) && !strings.Contains(agent, b.T("severity.info")) && !strings.Contains(agent, b.T("severity.critical")) {
		// severity line should be localized when assessment present
		if model.Agent.HasAssessment && model.Agent.Severity != "" {
			t.Fatalf("expected localized severity in agent screen:\n%s", agent)
		}
	}
	storage := Render(b, ScreenStorage, model)
	if !strings.Contains(storage, "н/д") {
		t.Fatalf("expected n/a for null SMART counters, got:\n%s", storage)
	}
	bitlocker := Render(b, ScreenBitLocker, model)
	if !strings.Contains(bitlocker, b.T("msg.bitlocker_unavailable")) {
		t.Fatalf("bitlocker unavailable missing:\n%s", bitlocker)
	}
}
