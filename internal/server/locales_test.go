package server

import (
	"testing"
)

func TestIsLanguageSupported(t *testing.T) {
	tests := []struct {
		lang string
		want bool
	}{
		{"en", true},
		{"zh", true},
		{"ja", true},
		{"ko", true},
		{"ar", true},
		{"fr", false},
		{"de", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := IsLanguageSupported(tt.lang); got != tt.want {
				t.Errorf("IsLanguageSupported(%q) = %v, want %v", tt.lang, got, tt.want)
			}
		})
	}
}

func TestGetTranslation(t *testing.T) {
	tests := []struct {
		lang string
		key  string
		want string
	}{
		{"en", "common.save", "Save"},
		{"zh", "common.save", "保存"},
		{"ja", "common.save", "保存"},
		{"ko", "common.save", "저장"},
		{"ar", "common.save", "حفظ"},
		{"en", "nonexistent.key", "nonexistent.key"},
		{"fr", "common.save", "Save"},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.key, func(t *testing.T) {
			if got := GetTranslation(tt.lang, tt.key); got != tt.want {
				t.Errorf("GetTranslation(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
			}
		})
	}
}

func TestTranslatef(t *testing.T) {
	result := Translatef("en", "time.minutes_ago", 5)
	expected := "5 mins ago"
	if result != expected {
		t.Errorf("Translatef() = %q, want %q", result, expected)
	}

	resultZh := Translatef("zh", "time.minutes_ago", 3)
	expectedZh := "3分钟前"
	if resultZh != expectedZh {
		t.Errorf("Translatef(zh) = %q, want %q", resultZh, expectedZh)
	}
}

func TestGetMissingTranslations(t *testing.T) {
	missing := GetMissingTranslations("ja")
	if len(missing) == 0 {
		t.Log("No missing translations for ja (may be complete)")
	} else {
		t.Logf("Found %d missing translations for ja", len(missing))
	}

	missingEn := GetMissingTranslations("en")
	if len(missingEn) > 0 {
		t.Errorf("English should have 0 missing translations, got %d", len(missingEn))
	}
}

func TestGetTranslationStats(t *testing.T) {
	stats := GetTranslationStats()
	if len(stats) < 2 {
		t.Errorf("Expected at least 2 languages, got %d", len(stats))
	}

	enCount, ok := stats["en"]
	if !ok || enCount == 0 {
		t.Error("Expected English translations to exist and have content")
	}

	t.Logf("Translation stats: %+v", stats)
}

func TestCheckPlaceholderConsistency(t *testing.T) {
	issues := CheckPlaceholderConsistency("en", "zh")
	if len(issues) > 0 {
		t.Logf("Found %d placeholder inconsistencies between en and zh", len(issues))
		for key, msgs := range issues {
			t.Logf("  %s: %v", key, msgs)
		}
	}
}

func TestCheckHTMLTags(t *testing.T) {
	issues := CheckHTMLTags("en")
	if len(issues) > 0 {
		t.Logf("Found %d HTML tag issues in en translations", len(issues))
		for key, msg := range issues {
			t.Logf("  %s: %s", key, msg)
		}
	}
}

func TestGetAllTranslationKeys(t *testing.T) {
	keys := GetAllTranslationKeys()
	if len(keys) == 0 {
		t.Error("Expected non-empty translation keys")
	}
	t.Logf("Total translation keys: %d", len(keys))
}

func TestExportImportTranslations(t *testing.T) {
	exported, err := ExportTranslations("en")
	if err != nil {
		t.Fatalf("ExportTranslations failed: %v", err)
	}
	if len(exported) == 0 {
		t.Error("Exported translations should not be empty")
	}

	testKey := "test.import.key"
	testValue := "Test Import Value"
	testData := TranslationMap{testKey: testValue}

	err = ImportTranslations("en", testData)
	if err != nil {
		t.Fatalf("ImportTranslations failed: %v", err)
	}

	result := GetTranslation("en", testKey)
	if result != testValue {
		t.Errorf("Imported translation not found, got %q, want %q", result, testValue)
	}
}

func TestGetLanguageInfo(t *testing.T) {
	info, ok := GetLanguageInfo("en")
	if !ok {
		t.Fatal("Expected English language info to exist")
	}
	if info.Code != "en" {
		t.Errorf("Expected code 'en', got %q", info.Code)
	}
	if info.RTL {
		t.Error("English should not be RTL")
	}

	infoAr, okAr := GetLanguageInfo("ar")
	if !okAr {
		t.Fatal("Expected Arabic language info to exist")
	}
	if !infoAr.RTL {
		t.Error("Arabic should be RTL")
	}

	_, okFr := GetLanguageInfo("fr")
	if okFr {
		t.Error("French should not be supported")
	}
}
