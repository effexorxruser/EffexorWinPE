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
	storage := Render(b, ScreenStorage, model)
	if !strings.Contains(storage, "н/д") {
		t.Fatalf("expected n/a for null SMART counters, got:\n%s", storage)
	}
	bitlocker := Render(b, ScreenBitLocker, model)
	if !strings.Contains(bitlocker, b.T("msg.bitlocker_unavailable")) {
		t.Fatalf("bitlocker unavailable missing:\n%s", bitlocker)
	}
}
