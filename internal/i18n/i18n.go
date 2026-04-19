package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// locale stores translations for a language
type locale map[string]string

// Translator provides internationalization support
type Translator struct {
	locales map[string]locale
	mu      sync.RWMutex
}

var defaultTranslator *Translator
var once sync.Once

// GetTranslator returns the singleton translator instance
func GetTranslator() *Translator {
	once.Do(func() {
		defaultTranslator = NewTranslator()
	})
	return defaultTranslator
}

// NewTranslator creates a new Translator and loads all locale files
func NewTranslator() *Translator {
	t := &Translator{
		locales: make(map[string]locale),
	}
	t.loadLocales()
	return t
}

// loadLocales loads all JSON locale files.
// It first tries the embedded filesystem (for release binaries),
// then falls back to reading from disk (for development/testing).
func (t *Translator) loadLocales() {
	// Try embedded filesystem first
	entries, err := localeFS.ReadDir("locales")
	if err == nil && len(entries) > 0 {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := localeFS.ReadFile("locales/" + entry.Name())
			if err != nil {
				continue
			}
			var m locale
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			lang := strings.TrimSuffix(entry.Name(), ".json")
			t.locales[lang] = m
		}
		return
	}

	// Fallback: load from disk (for development or when embed doesn't work)
	// Try to find locale dir relative to current working directory
	// os.Getwd() returns the directory go test was invoked from (project root or package dir)
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	// Try multiple possible locations
	searchPaths := []string{
		filepath.Join(cwd, "internal", "i18n", "locales"),
		filepath.Join(cwd, "..", "i18n", "locales"),
		filepath.Join(cwd, "..", "..", "internal", "i18n", "locales"),
	}

	var localeDir string
	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			localeDir = p
			break
		}
	}
	if localeDir == "" {
		return
	}
	entries, err = os.ReadDir(localeDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(localeDir, entry.Name()))
		if err != nil {
			continue
		}
		var m locale
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".json")
		t.locales[lang] = m
	}
}

// T returns the translation for the given language and key
// Falls back to English if the language or key is not found
func (t *Translator) T(lang, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Normalize lang code: "zh" → "zh-CN", "en" → "en-US"
	if lang == "zh" {
		lang = "zh-CN"
	} else if lang == "en" {
		lang = "en-US"
	}

	if l, ok := t.locales[lang]; ok {
		if v, ok := l[key]; ok {
			return v
		}
	}
	// fallback: try the other variant
	if lang == "zh-CN" {
		if l, ok := t.locales["en-US"]; ok {
			if v, ok := l[key]; ok {
				return v
			}
		}
	}
	// fallback to key itself
	return key
}

// T is a convenience function that uses the default translator
func T(lang, key string) string {
	return GetTranslator().T(lang, key)
}

// AvailableLanguages returns a list of available language codes
func (t *Translator) AvailableLanguages() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	langs := make([]string, 0, len(t.locales))
	for lang := range t.locales {
		langs = append(langs, lang)
	}
	return langs
}
