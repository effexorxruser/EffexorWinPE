package i18n

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/effexorxruser/EffexorWinPE/locales"
)

const (
	LocaleRuRU = "ru-RU"
	LocaleEnUS = "en-US"
	Default    = LocaleRuRU
	Fallback   = LocaleEnUS
)

// Bundle resolves UI strings by stable key.
type Bundle struct {
	locale   string
	primary  map[string]string
	fallback map[string]string
}

var (
	loadOnce sync.Once
	catalogs map[string]map[string]string
	loadErr  error
)

func loadCatalogs() {
	catalogs = make(map[string]map[string]string, 2)
	for _, name := range []string{LocaleRuRU, LocaleEnUS} {
		raw, err := locales.FS.ReadFile(name + ".json")
		if err != nil {
			loadErr = fmt.Errorf("read locale %s: %w", name, err)
			return
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			loadErr = fmt.Errorf("parse locale %s: %w", name, err)
			return
		}
		catalogs[name] = entries
	}
}

// New returns a bundle for locale. Unknown locales fall back to Default.
func New(locale string) (*Bundle, error) {
	loadOnce.Do(loadCatalogs)
	if loadErr != nil {
		return nil, loadErr
	}
	if _, ok := catalogs[locale]; !ok {
		locale = Default
	}
	return &Bundle{
		locale:   locale,
		primary:  catalogs[locale],
		fallback: catalogs[Fallback],
	}, nil
}

// Locale returns the active locale id.
func (b *Bundle) Locale() string {
	if b == nil {
		return Default
	}
	return b.locale
}

// T returns the translated string for key.
// Missing keys use en-US, then the key itself.
func (b *Bundle) T(key string) string {
	if b != nil {
		if v, ok := b.primary[key]; ok {
			return v
		}
		if b.fallback != nil {
			if v, ok := b.fallback[key]; ok {
				return v
			}
		}
	}
	loadOnce.Do(loadCatalogs)
	if catalogs != nil {
		if v, ok := catalogs[Fallback][key]; ok {
			return v
		}
	}
	return key
}

// Keys returns the key set for a locale catalog.
func Keys(locale string) (map[string]struct{}, error) {
	loadOnce.Do(loadCatalogs)
	if loadErr != nil {
		return nil, loadErr
	}
	src, ok := catalogs[locale]
	if !ok {
		return nil, fmt.Errorf("unknown locale %q", locale)
	}
	out := make(map[string]struct{}, len(src))
	for k := range src {
		out[k] = struct{}{}
	}
	return out, nil
}

// Catalog returns a copy of the locale map.
func Catalog(locale string) (map[string]string, error) {
	loadOnce.Do(loadCatalogs)
	if loadErr != nil {
		return nil, loadErr
	}
	src, ok := catalogs[locale]
	if !ok {
		return nil, fmt.Errorf("unknown locale %q", locale)
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out, nil
}
