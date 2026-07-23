package i18n

import (
	"strings"
	"testing"
)

func TestKeyParityRuEn(t *testing.T) {
	ru, err := Keys(LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	en, err := Keys(LocaleEnUS)
	if err != nil {
		t.Fatal(err)
	}
	if len(ru) == 0 {
		t.Fatal("ru-RU catalog is empty")
	}
	for k := range ru {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q present in ru-RU but missing in en-US", k)
		}
	}
	for k := range en {
		if _, ok := ru[k]; !ok {
			t.Errorf("key %q present in en-US but missing in ru-RU", k)
		}
	}
}

func TestFallbackMissingKey(t *testing.T) {
	b, err := New(LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	// Force a missing primary key by using a synthetic bundle.
	synthetic := &Bundle{
		locale:   LocaleRuRU,
		primary:  map[string]string{},
		fallback: map[string]string{"label.diagnostics": "Diagnostics"},
	}
	if got := synthetic.T("label.diagnostics"); got != "Diagnostics" {
		t.Fatalf("fallback = %q, want Diagnostics", got)
	}
	if got := b.T("definitely.missing.key"); got != "definitely.missing.key" {
		t.Fatalf("missing key fallback = %q", got)
	}
}

func TestDefaultLocaleRussian(t *testing.T) {
	b, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if b.Locale() != Default {
		t.Fatalf("locale = %q, want %q", b.Locale(), Default)
	}
	if got := b.T("label.diagnostics"); got != "Диагностика" {
		t.Fatalf("diagnostics label = %q", got)
	}
}

func TestRequiredTerminology(t *testing.T) {
	b, err := New(LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"label.diagnostics":           "Диагностика",
		"label.data_collection":       "Сбор данных",
		"label.results":               "Результаты",
		"label.windows_installs":      "Обнаруженные установки Windows",
		"label.storage_health":        "Состояние накопителей",
		"label.bitlocker":             "Шифрование BitLocker",
		"label.network_adapters":      "Сетевые адаптеры",
		"label.ethernet":              "Подключение Ethernet",
		"msg.data_source_unavailable": "Источник данных недоступен",
		"msg.volume_count_unknown":    "Количество томов неизвестно",
		"action.export_report":        "Экспортировать отчёт",
		"action.open_journal":         "Открыть журнал",
		"action.open_cmd":             "Открыть командную строку",
		"action.reboot":               "Перезагрузить компьютер",
		"action.shutdown":             "Завершить работу",
	}
	for key, want := range cases {
		if got := b.T(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestUTF16RoundTripCyrillic(t *testing.T) {
	b, err := New(LocaleRuRU)
	if err != nil {
		t.Fatal(err)
	}
	text := b.T("app.title")
	encoded := EncodeUTF16(text)
	if len(encoded) < 2 || encoded[len(encoded)-1] != 0 {
		t.Fatalf("expected null-terminated utf16, got %#v", encoded)
	}
	back := DecodeUTF16(encoded)
	if back != text {
		t.Fatalf("round-trip = %q, want %q", back, text)
	}
	if !strings.Contains(text, "диагностика") && !strings.Contains(strings.ToLower(text), "диагностика") {
		// Title contains Cyrillic word "диагностика" with capital Д.
		if !strings.Contains(text, "Диагностика") && !strings.Contains(text, "диагностика") {
			t.Fatalf("expected Cyrillic in title, got %q", text)
		}
	}
}

func TestNoANPInCatalogs(t *testing.T) {
	for _, locale := range []string{LocaleRuRU, LocaleEnUS} {
		cat, err := Catalog(locale)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range cat {
			if strings.Contains(strings.ToUpper(k), "ANP") || strings.Contains(strings.ToUpper(v), "ANP") {
				t.Errorf("ANP mention in %s: %s=%q", locale, k, v)
			}
		}
	}
}
