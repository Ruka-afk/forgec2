package server

import (
	"encoding/csv"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Pre-compiled regex patterns for credential parsing (performance optimization)
var (
	credReBlock    *regexp.Regexp
	credReDomain   *regexp.Regexp
	credReNTLM     *regexp.Regexp
	credReSHA1     *regexp.Regexp
	credRePassword *regexp.Regexp
	credReSAM      *regexp.Regexp
	credOnce       sync.Once
)

func initCredRegexps() {
	credOnce.Do(func() {
		credReBlock = regexp.MustCompile(`(?i)(?:Username|User)\s*:\s*(.+)`)
		credReDomain = regexp.MustCompile(`(?i)Domain\s*:\s*(.+)`)
		credReNTLM = regexp.MustCompile(`(?i)NTLM\s*:\s*([a-fA-F0-9]{32})`)
		credReSHA1 = regexp.MustCompile(`(?i)SHA1?\s*:\s*([a-fA-F0-9]{40})`)
		credRePassword = regexp.MustCompile(`(?i)Password\s*:\s*(.+?)\s*$`)
		credReSAM = regexp.MustCompile(`^([^\s:]+):(\d+):([a-fA-F0-9]{32}):([a-fA-F0-9]{32}):::`)
	})
}

// parseAndStoreCredentials parses common credential dump formats (mimikatz-style)
// and stores extracted entries in the credential vault.
func parseAndStoreCredentials(database *gorm.DB, agentID string, raw string, taskID uint) {
	entries := parseCredentialsFromText(raw, agentID, taskID)
	if len(entries) == 0 {
		return
	}
	
	// Optimization: Load existing creds once and use HashSet for O(1) lookup
	type credKey struct {
		AgentID, Domain, Username, Hash, Password string
	}
	
	var existing []db.CredentialEntry
	database.Where("agent_id = ?", agentID).Find(&existing)
	
	existingSet := make(map[credKey]bool, len(existing))
	for _, e := range existing {
		existingSet[credKey{e.AgentID, e.Domain, e.Username, e.Hash, e.Password}] = true
	}
	
	// Filter duplicates using HashSet
	var batch []db.CredentialEntry
	for _, e := range entries {
		key := credKey{e.AgentID, e.Domain, e.Username, e.Hash, e.Password}
		if !existingSet[key] {
			batch = append(batch, e)
			existingSet[key] = true // Mark as added to avoid duplicates in batch
		}
	}
	
	if len(batch) > 0 {
		database.CreateInBatches(batch, 50)
		slog.Info("Credentials stored in vault", "agent", agentID, "count", len(batch))
	}
}

// parseAndStoreKerberoastResults parses kerberoast TGS hash output (SPN:HASH)
// and stores entries in the credential vault.
func parseAndStoreKerberoastResults(database *gorm.DB, agentID string, raw string, taskID uint) {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: SPN:HASH (kerberoast output)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		spn := strings.TrimSpace(parts[0])
		hash := strings.TrimSpace(parts[1])
		if spn == "" || hash == "" {
			continue
		}
		// Extract user and domain from SPN (user@domain or service/domain)
		user := spn
		domain := ""
		if atIdx := strings.Index(spn, "@"); atIdx > 0 {
			user = spn[:atIdx]
			domain = spn[atIdx+1:]
		} else if slashIdx := strings.Index(spn, "/"); slashIdx > 0 {
			user = spn[slashIdx+1:]
			domain = spn[:slashIdx]
		}

		entry := db.CredentialEntry{
			AgentID:  agentID,
			Domain:   domain,
			Username: user,
			Hash:     hash,
			Source:   "kerberoast",
			Type:     "krb_tgs",
			Notes:    "SPN: " + spn,
			TaskID:   taskID,
		}
		// Avoid exact duplicates
		var count int64
		database.Model(&db.CredentialEntry{}).
			Where("agent_id = ? AND domain = ? AND username = ? AND hash = ? AND source = ?",
				entry.AgentID, entry.Domain, entry.Username, entry.Hash, entry.Source).Count(&count)
		if count == 0 {
			database.Create(&entry)
			slog.Info("Kerberoast hash stored in vault", "agent", agentID, "spn", spn)
		}
	}
}

// parseCredentialsFromText handles multiple output formats
func parseCredentialsFromText(raw string, agentID string, taskID uint) []db.CredentialEntry {
	initCredRegexps() // Ensure regexps are compiled
	var entries []db.CredentialEntry

	// Pattern 1: mimikatz sekurlsa::logonpasswords style
	blocks := regexp.MustCompile(`\n\s*\n`).Split(raw, -1)
	for _, block := range blocks {
		var entry db.CredentialEntry
		entry.AgentID = agentID
		entry.TaskID = taskID
		entry.Source = "mimikatz"

		if m := credReBlock.FindStringSubmatch(block); len(m) > 1 {
			entry.Username = strings.TrimSpace(m[1])
		}
		if m := credReDomain.FindStringSubmatch(block); len(m) > 1 {
			entry.Domain = strings.TrimSpace(m[1])
		}
		if m := credReNTLM.FindStringSubmatch(block); len(m) > 1 {
			entry.Hash = strings.TrimSpace(m[1])
			entry.Type = "ntlm"
		}
		if m := credReSHA1.FindStringSubmatch(block); len(m) > 1 && entry.Hash == "" {
			entry.Hash = strings.TrimSpace(m[1])
			entry.Type = "sha1"
		}
		if m := credRePassword.FindStringSubmatch(block); len(m) > 1 {
			pw := strings.TrimSpace(m[1])
			if pw != "(null)" && pw != "" {
				entry.Password = pw
				if entry.Type == "" {
					entry.Type = "cleartext"
				}
			}
		}

		if entry.Username != "" && entry.Username != "(null)" && (entry.Hash != "" || entry.Password != "") {
			entries = append(entries, entry)
		}
	}

	// Pattern 2: SAM hash dump format — username:rid:lmhash:nthash:::
	for _, line := range strings.Split(raw, "\n") {
		if m := credReSAM.FindStringSubmatch(strings.TrimSpace(line)); len(m) > 4 {
			entries = append(entries, db.CredentialEntry{
				AgentID:  agentID,
				Username: m[1],
				Hash:     m[4],
				Source:   "sam",
				Type:     "ntlm",
				TaskID:   taskID,
			})
		}
	}

	// Pattern 3: Simple domain\user:password or user:password lines
	reSimple := regexp.MustCompile(`^(?:([^\s:\\]+)\\)?([^\s:]+):(.+)$`)
	if len(entries) == 0 {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if m := reSimple.FindStringSubmatch(line); len(m) > 3 {
				domain := strings.TrimSpace(m[1])
				user := strings.TrimSpace(m[2])
				pw := strings.TrimSpace(m[3])
				if strings.Contains(pw, "/") || strings.Contains(pw, "\\") || len(pw) > 256 {
					continue
				}
				entries = append(entries, db.CredentialEntry{
					AgentID:  agentID,
					Domain:   domain,
					Username: user,
					Password: pw,
					Source:   "manual_parse",
					Type:     "cleartext",
					TaskID:   taskID,
				})
			}
		}
	}

	return entries
}

// handleCredentialsPage renders the credentials vault page (DB-backed)
func (s *Server) handleCredentialsPage(c *gin.Context) {
	var creds []db.CredentialEntry
	s.db.Order("created_at desc").Limit(500).Find(&creds)

	// Raw creds tasks for backward compat
	var credsTasks []db.Task
	s.db.Preload("Agent").
		Where("type = ?", "creds").
		Order("created_at desc").Limit(100).Find(&credsTasks)

	var related []db.Task
	s.db.Preload("Agent").
		Where("type = ? AND (command LIKE ? OR command LIKE ? OR command LIKE ?)", "shell", "%mimikatz%", "%sekurlsa%", "%lsass%").
		Order("created_at desc").Limit(30).Find(&related)

	stats := s.getNavStats()
	data := gin.H{
		"Title":        "ForgeC2 - Credential Center",
		"ActiveNav":    "credentials",
		"VaultEntries": creds,
		"CredsTasks":   credsTasks,
		"RelatedTasks": related,
		"VaultCount":   len(creds),
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "credentials_content", data)
}

func (s *Server) handleExportCredentials(c *gin.Context) {
	var creds []db.CredentialEntry
	s.db.Order("created_at desc").Find(&creds)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=credentials.csv")

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"ID", "AgentID", "Domain", "Username", "Password", "Hash", "Source", "Type", "Created"})
	for _, e := range creds {
		w.Write([]string{
			strconv.FormatUint(uint64(e.ID), 10),
			e.AgentID, e.Domain, e.Username, e.Password, e.Hash,
			e.Source, e.Type, e.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
}

func (s *Server) handleAddCredential(c *gin.Context) {
	entry := db.CredentialEntry{
		AgentID:  c.PostForm("agent_id"),
		Domain:   c.PostForm("domain"),
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
		Hash:     c.PostForm("hash"),
		Source:   "manual",
		Type:     c.PostForm("type"),
		Notes:    c.PostForm("notes"),
	}
	if entry.Type == "" {
		if entry.Hash != "" {
			entry.Type = "ntlm"
		} else {
			entry.Type = "cleartext"
		}
	}
	if err := s.db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add credential"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "id": entry.ID})
}

func (s *Server) handleDeleteCredential(c *gin.Context) {
	idStr := c.Param("cred_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := s.db.Delete(&db.CredentialEntry{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}


