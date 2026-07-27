package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleSetLanguage(c *gin.Context) {
	lang := c.PostForm("lang")
	if lang == "" {
		lang = c.Query("lang")
	}
	if lang == "" {
		respondError(c, http.StatusBadRequest, "Language code is required")
		return
	}

	if !IsLanguageSupported(lang) {
		respondError(c, http.StatusBadRequest, "Unsupported language")
		return
	}

	middleware.SetCookieWithSameSite(c, "forgec2_lang", lang, LangCookieMaxAgeSec, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)

	referer := c.GetHeader("Referer")
	if referer != "" && strings.HasPrefix(referer, "/") && !strings.HasPrefix(referer, "//") {
		if parsedURL, err := url.Parse(referer); err == nil && parsedURL.Host == "" {
			c.Redirect(http.StatusFound, referer)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"language": lang,
		"message":  "Language updated",
	})
}

func (s *Server) handleTranslationsPage(c *gin.Context) {
	s.renderPageOrJSON(c, gin.H{
		"Title":     "Translation Management",
		"ActiveNav": "translations",
	})
}

func (s *Server) handleDocsPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/api/docs/")
}

func (s *Server) handleGetTranslations(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = detectLanguage(c)
	}

	translations, err := ExportTranslations(lang)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Settings save"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"language":     lang,
		"translations": translations,
		"count":        len(translations),
	})
}

func (s *Server) handleTranslationStats(c *gin.Context) {
	stats := GetTranslationStats()

	missing := make(map[string][]string)
	for lang := range SupportedLanguages {
		missing[lang] = GetMissingTranslations(lang)
	}

	allKeys := GetAllTranslationKeys()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"languages":  SupportedLanguages,
		"stats":      stats,
		"total_keys": len(allKeys),
		"missing":    missing,
	})
}

func (s *Server) handleTranslationCheck(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = detectLanguage(c)
	}

	missing := GetMissingTranslations(lang)
	placeholderIssues := CheckPlaceholderConsistency(DefaultLanguage, lang)
	htmlIssues := CheckHTMLTags(lang)

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"language":             lang,
		"missing_translations": missing,
		"missing_count":        len(missing),
		"placeholder_issues":   placeholderIssues,
		"html_tag_issues":      htmlIssues,
	})
}
