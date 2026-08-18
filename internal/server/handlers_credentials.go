package server

import (
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Pre-compiled regex patterns for credential parsing (performance optimization)
var (
	credReBlock       *regexp.Regexp
	credReDomain      *regexp.Regexp
	credReNTLM        *regexp.Regexp
	credReSHA1        *regexp.Regexp
	credRePassword    *regexp.Regexp
	credReSAM         *regexp.Regexp
	credReSplitBlocks *regexp.Regexp
	credReSimple      *regexp.Regexp
	credOnce          sync.Once
)

func initCredRegexps() {
	credOnce.Do(func() {
		credReBlock = regexp.MustCompile(`(?i)(?:Username|User)\s*:\s*(.+)`)
		credReDomain = regexp.MustCompile(`(?i)Domain\s*:\s*(.+)`)
		credReNTLM = regexp.MustCompile(`(?i)NTLM\s*:\s*([a-fA-F0-9]{32})`)
		credReSHA1 = regexp.MustCompile(`(?i)SHA1?\s*:\s*([a-fA-F0-9]{40})`)
		credRePassword = regexp.MustCompile(`(?i)Password\s*:\s*(.+?)\s*$`)
		credReSAM = regexp.MustCompile(`^([^\s:]+):(\d+):([a-fA-F0-9]{32}):([a-fA-F0-9]{32}):::`)
		credReSplitBlocks = regexp.MustCompile(`\n\s*\n`)
		credReSimple = regexp.MustCompile(`^(?:([^\s:\\]+)\\)?([^\s:]+):(.+)$`)
	})
}

// splitASREPRoastLine parses one agent asreproast line
// ("$krb5asrep$<etype>$<user>@<realm>:<cipherhex>") into user and realm. The
// agent requests RC4-only tickets (etype 23), so the stored line is directly
// hashcat -m 18200 crackable; any other valid etype is kept as an artifact.
func splitASREPRoastLine(line string) (user string, realm string, ok bool) {
	if !strings.HasPrefix(line, "$krb5asrep$") {
		return "", "", false
	}
	rest := line[len("$krb5asrep$"):]
	etypeEnd := strings.Index(rest, "$")
	if etypeEnd <= 0 {
		return "", "", false
	}
	for _, c := range rest[:etypeEnd] {
		if c < '0' || c > '9' {
			return "", "", false
		}
	}
	acct := rest[etypeEnd+1:]
	at := strings.Index(acct, "@")
	colon := strings.LastIndex(acct, ":")
	if at <= 0 || colon <= at+1 {
		return "", "", false
	}
	if len(acct[colon+1:]) < 16 {
		return "", "", false
	}
	return acct[:at], acct[at+1 : colon], true
}

// parseAndStoreASREPRoastResults ingests agent asreproast output into the
// credential vault. The agent emits one hashcat -m 18200 line per Roastable
// account; failure notes ("[!] ...") and foreign lines are skipped. The stored
// hash is the full line so operators can paste it straight into hashcat.
func (s *Server) parseAndStoreASREPRoastResults(agentID string, raw string, taskID uint) {
	database := s.db
	lines := strings.Split(raw, "\n")
	type credKey struct {
		AgentID, Domain, Username, Hash, Source string
	}
	var existing []db.CredentialEntry
	database.Model(&db.CredentialEntry{}).
		Select("agent_id, domain, username, hash").
		Where("agent_id = ? AND source = 'asreproast'", agentID).
		Find(&existing)
	existSet := make(map[credKey]bool, len(existing))
	for _, k := range existing {
		existSet[credKey{AgentID: agentID, Domain: k.Domain, Username: k.Username, Hash: k.Hash, Source: "asreproast"}] = true
	}

	var newEntries []db.CredentialEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[!]") {
			continue
		}
		user, realm, ok := splitASREPRoastLine(line)
		if !ok {
			continue
		}
		k := credKey{AgentID: agentID, Domain: realm, Username: user, Hash: line, Source: "asreproast"}
		if existSet[k] {
			continue
		}
		existSet[k] = true
		newEntries = append(newEntries, db.CredentialEntry{
			AgentID: agentID, Domain: realm, Username: user, Hash: line,
			Source: "asreproast", Type: "krb_asrep", Notes: "AS-REP: " + user + "@" + realm, TaskID: taskID,
		})
	}
	if len(newEntries) > 0 {
		if err := database.Create(&newEntries).Error; err != nil {
			slog.Error("Failed to store asreproast hashes", "agent_id", agentID, "err", err)
		} else {
			slog.Info("AS-REP roast hashes stored in vault", "agent_id", agentID, "count", len(newEntries))
			s.LogAuditRecord(nil, "credential_ingest", "credential", agentID,
				"stored "+strconv.Itoa(len(newEntries))+" asreproast hashes", true, nil)
			s.broadcastOperatorEvent(map[string]interface{}{
				"type":     "credential_update",
				"action":   "found",
				"agent_id": agentID,
				"count":    len(newEntries),
			})
		}
	}
}

// parseAndStoreCredentials parses common credential dump formats (mimikatz-style)
// and stores extracted entries in the credential vault.
func (s *Server) parseAndStoreCredentials(agentID string, raw string, taskID uint) {
	database := s.db
	entries := parseCredentialsFromText(raw, agentID, taskID)
	if len(entries) == 0 {
		return
	}

	// Optimization: Load existing creds once and use HashSet for O(1) lookup
	type credKey struct {
		AgentID, Domain, Username, Hash, Password string
	}

	existingSet := make(map[credKey]bool)
	var lastID uint
	const batchSize = 1000
	for {
		var batch []db.CredentialEntry
		if err := database.Where("agent_id = ? AND id > ?", agentID, lastID).
			Order("id ASC").Limit(batchSize).Find(&batch).Error; err != nil {
			slog.Error("Failed to load existing credential batch", "err", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, e := range batch {
			existingSet[credKey{e.AgentID, e.Domain, e.Username, e.Hash, e.Password}] = true
		}
		lastID = batch[len(batch)-1].ID
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
		if err := database.CreateInBatches(batch, 50).Error; err != nil {
			slog.Error("Failed to store credentials batch", "agent_id", agentID, "err", err)
		} else {
			slog.Info("Credentials stored in vault", "agent_id", agentID, "count", len(batch))
			s.LogAuditRecord(nil, "credential_ingest", "credential", agentID,
				"stored "+strconv.Itoa(len(batch))+" credentials (source="+parseCredentialSource(raw)+")", true, nil)
			// Push the vault change to open dashboard sessions so the
			// Credentials page refreshes without polling.
			s.broadcastOperatorEvent(map[string]interface{}{
				"type":     "credential_update",
				"action":   "found",
				"agent_id": agentID,
				"count":    len(batch),
			})
		}
	}
}

// parseCredentialSource classifies a raw credential dump for audit detail
// strings. The raw bytes are never logged.
func parseCredentialSource(raw string) string {
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "krb5tgs") || strings.Contains(low, "$krb5tgs"):
		return "kerberoast"
	case strings.Contains(low, "sekurlsa") || strings.Contains(low, "logonpasswords"):
		return "mimikatz"
	default:
		return "dump"
	}
}

// splitKerberoastLine splits one kerberoast output line into
// (user, domain, spn, hash). Two formats are accepted:
//   - legacy "SPN:HASH" from older implants (user/domain derived from the SPN),
//   - hashcat-mode lines "$krb5tgs$23$*user$realm$spn*$checksum$edata2" emitted
//     by the agent's DER converter; the account segment carries user, realm and
//     spn verbatim and the full line is kept as the stored hash so it can be
//     dropped straight into hashcat -m 13100 (or 19600 for etype 18).
func splitKerberoastLine(line string) (user string, domain string, spn string, hash string, ok bool) {
	if strings.HasPrefix(line, "$krb5tgs$") {
		// "$krb5tgs$23$*user$realm$spn*$checksum$edata2" — the account segment
		// is delimited by the leading "$*" and the trailing "*$".
		acctStart := strings.Index(line, "$*")
		if acctStart < 0 {
			return "", "", "", "", false
		}
		acctBody := line[acctStart+2:]
		acctEnd := strings.Index(acctBody, "*$")
		if acctEnd < 0 {
			return "", "", "", "", false
		}
		ap := strings.Split(acctBody[:acctEnd], "$")
		if len(ap) != 3 || ap[0] == "" || ap[1] == "" || ap[2] == "" {
			return "", "", "", "", false
		}
		payload := acctBody[acctEnd+2:]
		sep := strings.Index(payload, "$")
		if sep != 32 || len(payload) <= sep+1 {
			return "", "", "", "", false
		}
		return ap[0], ap[1], ap[2], line, true
	}
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", "", "", false
	}
	spn = strings.TrimSpace(parts[0])
	hash = strings.TrimSpace(parts[1])
	if spn == "" || hash == "" {
		return "", "", "", "", false
	}
	user = spn
	if atIdx := strings.Index(spn, "@"); atIdx > 0 {
		user = spn[:atIdx]
		domain = spn[atIdx+1:]
	} else if slashIdx := strings.Index(spn, "/"); slashIdx > 0 {
		user = spn[slashIdx+1:]
		domain = spn[:slashIdx]
	}
	return user, domain, spn, hash, true
}

// parseAndStoreKerberoastResults parses kerberoast TGS hash output (SPN:HASH
// or hashcat-mode lines) and stores entries in the credential vault.
func (s *Server) parseAndStoreKerberoastResults(agentID string, raw string, taskID uint) {
	database := s.db
	lines := strings.Split(raw, "\n")
	// Batch-load existing hashes to avoid N+1 queries. Scan into the full
	// model (not an anonymous struct) so AfterFind transparently decrypts
	// the stored hash and the dedup key compares plaintext.
	type credKey struct {
		AgentID, Domain, Username, Hash, Source string
	}
	var existing []db.CredentialEntry
	database.Model(&db.CredentialEntry{}).
		Select("agent_id, domain, username, hash").
		Where("agent_id = ? AND source = 'kerberoast'", agentID).
		Find(&existing)
	existSet := make(map[credKey]bool, len(existing))
	for _, k := range existing {
		existSet[credKey{AgentID: agentID, Domain: k.Domain, Username: k.Username, Hash: k.Hash, Source: "kerberoast"}] = true
	}

	var newEntries []db.CredentialEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		user, domain, spn, hash, ok := splitKerberoastLine(line)
		if !ok {
			continue
		}
		k := credKey{AgentID: agentID, Domain: domain, Username: user, Hash: hash, Source: "kerberoast"}
		if existSet[k] {
			continue
		}
		existSet[k] = true
		newEntries = append(newEntries, db.CredentialEntry{
			AgentID: agentID, Domain: domain, Username: user, Hash: hash,
			Source: "kerberoast", Type: "krb_tgs", Notes: "SPN: " + spn, TaskID: taskID,
		})
	}
	if len(newEntries) > 0 {
		if err := database.Create(&newEntries).Error; err != nil {
			slog.Error("Failed to store kerberoast hashes", "agent_id", agentID, "err", err)
		} else {
			slog.Info("Kerberoast hashes stored in vault", "agent_id", agentID, "count", len(newEntries))
			s.LogAuditRecord(nil, "credential_ingest", "credential", agentID,
				"stored "+strconv.Itoa(len(newEntries))+" kerberoast hashes", true, nil)
			// Push the vault change to open dashboard sessions so the
			// Credentials page refreshes without polling.
			s.broadcastOperatorEvent(map[string]interface{}{
				"type":     "credential_update",
				"action":   "found",
				"agent_id": agentID,
				"count":    len(newEntries),
			})
		}
	}
}

// parseAndStorePasswordSprayResults ingests valid password spray hits into
// the credential vault. The spray output is JSON with per-user status; the
// sprayed password and domain travel in the agent-side task (Command) and are
// copied into the stored entries. Only status "valid" hits are stored (they
// were verified by LogonUser on the agent), locked/erroneous results are
// discarded. Returns the number of new entries stored.
func (s *Server) parseAndStorePasswordSprayResults(agentID string, task db.Task, raw string) int {
	if task.Command == "" || raw == "" {
		return 0
	}
	cmdParts := strings.SplitN(task.Command, "|", 4)
	if len(cmdParts) < 2 {
		return 0
	}
	password := cmdParts[0]
	domain := cmdParts[1]

	var out struct {
		Results []struct {
			User   string `json:"user"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out.Results) == 0 {
		return 0
	}

	type credKey struct {
		AgentID, Domain, Username, Password, Source string
	}
	var existing []db.CredentialEntry
	s.db.Model(&db.CredentialEntry{}).
		Select("domain, username, password").
		Where("agent_id = ? AND source = ?", agentID, "password_spray").
		Find(&existing)
	existSet := make(map[credKey]bool, len(existing))
	for _, k := range existing {
		existSet[credKey{agentID, k.Domain, k.Username, k.Password, "password_spray"}] = true
	}

	var newEntries []db.CredentialEntry
	for _, r := range out.Results {
		if r.Status != "valid" {
			continue
		}
		entryUser := strings.TrimSpace(r.User)
		if entryUser == "" {
			continue
		}
		entryDomain := domain
		if atIdx := strings.Index(entryUser, "@"); atIdx > 0 {
			if entryDomain == "" {
				entryDomain = entryUser[atIdx+1:]
			}
			entryUser = entryUser[:atIdx]
		}
		if slashIdx := strings.Index(entryUser, "\\"); slashIdx > 0 {
			if entryDomain == "" {
				entryDomain = entryUser[:slashIdx]
			}
			entryUser = entryUser[slashIdx+1:]
		}
		if entryUser == "" {
			continue
		}
		k := credKey{agentID, entryDomain, entryUser, password, "password_spray"}
		if existSet[k] {
			continue
		}
		existSet[k] = true
		newEntries = append(newEntries, db.CredentialEntry{
			AgentID:   agentID,
			Domain:    entryDomain,
			Username:  entryUser,
			Password:  password,
			Source:    "password_spray",
			Type:      "cleartext",
			Confirmed: true, // verified by LogonUser during the spray
			Notes:     "validated via password spray",
			TaskID:    task.ID,
		})
	}

	if len(newEntries) == 0 {
		return 0
	}
	if err := s.db.Create(&newEntries).Error; err != nil {
		slog.Error("Failed to store password spray credentials", "agent_id", agentID, "err", err)
		return 0
	}
	slog.Info("Password spray hits stored in vault", "agent_id", agentID, "count", len(newEntries))
	s.LogAuditRecord(nil, "credential_ingest", "credential", agentID,
		"stored "+strconv.Itoa(len(newEntries))+" password spray credentials", true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":     "credential_update",
		"action":   "found",
		"agent_id": agentID,
		"count":    len(newEntries),
	})
	return len(newEntries)
}

// parseCredentialsFromText handles multiple output formats
func parseCredentialsFromText(raw string, agentID string, taskID uint) []db.CredentialEntry {
	initCredRegexps() // Ensure regexps are compiled
	var entries []db.CredentialEntry

	// Pattern 1: mimikatz sekurlsa::logonpasswords style
	blocks := credReSplitBlocks.Split(raw, -1)
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

	// Pattern 2: SAM hash dump format 鈥?username:rid:lmhash:nthash:::
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
	if len(entries) == 0 {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if m := credReSimple.FindStringSubmatch(line); len(m) > 3 {
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
	query := s.db.Model(&db.CredentialEntry{}).Order("created_at desc")

	tagFilter := c.Query("tag")
	searchQuery := c.Query("search")
	expiryFilter := c.Query("expiry")
	confirmedFilter := c.Query("confirmed")
	agentFilter := c.Query("agent_id")

	if agentFilter != "" {
		query = query.Where("agent_id = ?", agentFilter)
	}

	if tagFilter != "" {
		query = query.Where("tags LIKE ? ESCAPE '\\'", "%"+escapeLike(tagFilter)+"%")
	}

	if searchQuery != "" {
		query = query.Where("(domain LIKE ? ESCAPE '\\' OR username LIKE ? ESCAPE '\\' OR notes LIKE ? ESCAPE '\\')",
			"%"+escapeLike(searchQuery)+"%", "%"+escapeLike(searchQuery)+"%", "%"+escapeLike(searchQuery)+"%")
	}

	if expiryFilter != "" {
		// Compare against Go-derived timestamps instead of SQLite
		// datetime('now'): stored times carry the local offset (e.g. +08:00)
		// while datetime('now') is UTC, making naive string comparison
		// timezone-dependent. Passing time.Time parameters keeps both sides
		// on the same storage format.
		now := time.Now()
		weekAhead := now.Add(7 * 24 * time.Hour)
		switch expiryFilter {
		case "expired":
			query = query.Where("expires_at IS NOT NULL AND expires_at < ?", now)
		case "expiring":
			query = query.Where("expires_at IS NOT NULL AND expires_at BETWEEN ? AND ?", now, weekAhead)
		case "valid":
			query = query.Where("expires_at IS NULL OR expires_at > ?", now)
		}
	}

	if confirmedFilter != "" {
		switch confirmedFilter {
		case "true":
			query = query.Where("confirmed = ?", true)
		case "false":
			query = query.Where("confirmed = ?", false)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		handleQueryError(c, err, "Failed to count credentials page")
		return
	}

	limit := 5000
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= limit {
			limit = n
		}
	}

	if err := query.Limit(limit).Find(&creds).Error; err != nil {
		handleQueryError(c, err, "Failed to query credentials page")
		return
	}

	for i := range creds {
		creds[i].Notes = decryptCredNotes(creds[i].Notes)
	}

	var allTags []string
	var tagStrings []string
	if err := s.db.Model(&db.CredentialEntry{}).Where("tags != '' AND tags IS NOT NULL").Limit(5000).Pluck("tags", &tagStrings).Error; err != nil {
		slog.Error("Failed to pluck credential tags", "err", err)
		tagStrings = []string{}
	}
	tagSet := make(map[string]int)
	for _, tags := range tagStrings {
		for _, tag := range strings.Split(tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagSet[tag]++
			}
		}
	}
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}

	var credsTasks []db.Task
	if err := s.db.Preload("Agent").
		Where("type = ?", "creds").
		Order("created_at desc").Limit(100).Find(&credsTasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query creds tasks")
		return
	}

	var related []db.Task
	if err := s.db.Preload("Agent").
		Where("type = ? AND (command LIKE ? OR command LIKE ? OR command LIKE ?)", "shell", "%mimikatz%", "%sekurlsa%", "%lsass%").
		Order("created_at desc").Limit(30).Find(&related).Error; err != nil {
		handleQueryError(c, err, "Failed to query related creds tasks")
		return
	}

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":        "ForgeC2 - Credential Center",
		"ActiveNav":    "credentials",
		"VaultEntries": creds,
		"CredsTasks":   credsTasks,
		"RelatedTasks": related,
		"VaultCount":   len(creds),
		"Total":        int(total),
		"AllTags":      allTags,
		"TagFilter":    tagFilter,
		"search_query": searchQuery,
		"ExpiryFilter": expiryFilter,
		"AgentFilter":  agentFilter,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// exportRateLimit tracks credential export requests per IP to prevent abuse.
var (
	exportRateMu     sync.Mutex
	exportRateMap    = make(map[string][]time.Time)
	exportRateLimit  = 5           // max exports per window
	exportRateWindow = time.Minute // sliding window
)

func checkExportRateLimit(ip string) bool {
	exportRateMu.Lock()
	defer exportRateMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-exportRateWindow)
	// Prune old entries
	timestamps := exportRateMap[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= exportRateLimit {
		exportRateMap[ip] = valid
		return false
	}
	exportRateMap[ip] = append(valid, now)
	// Amortized cleanup: prune IPs whose entries are all expired so the
	// map cannot grow unbounded with abandoned addresses.
	if len(exportRateMap) > 1024 && len(exportRateMap)%64 == 0 {
		for k, ts := range exportRateMap {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(exportRateMap, k)
			}
		}
	}
	return true
}

func (s *Server) handleExportCredentials(c *gin.Context) {
	if !checkExportRateLimit(c.ClientIP()) {
		respondError(c, http.StatusTooManyRequests, "export rate limit exceeded, try again later")
		return
	}

	s.LogAuditRecord(c, "credential_export", "credential", "", "credentials exported as CSV", true, nil)

	var creds []db.CredentialEntry
	query := s.db.Order("created_at desc").Limit(5000)

	tagFilter := c.Query("tag")
	expiryFilter := c.Query("expiry")

	if tagFilter != "" {
		query = query.Where("tags LIKE ? ESCAPE '\\'", "%"+escapeLike(tagFilter)+"%")
	}

	if expiryFilter != "" {
		now := time.Now()
		weekAhead := now.Add(7 * 24 * time.Hour)
		switch expiryFilter {
		case "expired":
			query = query.Where("expires_at IS NOT NULL AND expires_at < ?", now)
		case "expiring":
			query = query.Where("expires_at IS NOT NULL AND expires_at BETWEEN ? AND ?", now, weekAhead)
		case "valid":
			query = query.Where("expires_at IS NULL OR expires_at > ?", now)
		}
	}

	if err := query.Limit(5000).Find(&creds).Error; err != nil {
		slog.Error("Failed to query credentials for export", "err", err)
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=credentials.csv")

	w := csv.NewWriter(c.Writer)
	w.Write([]string{"ID", "AgentID", "Domain", "Username", "Password", "Hash", "Source", "Type", "Tags", "ExpiresAt", "Confirmed", "Created"})
	for _, e := range creds {
		w.Write([]string{
			strconv.FormatUint(uint64(e.ID), 10),
			csvSanitize(e.AgentID), csvSanitize(e.Domain), csvSanitize(e.Username),
			csvSanitize(e.Password), csvSanitize(e.Hash),
			csvSanitize(e.Source), csvSanitize(e.Type), csvSanitize(e.Tags),
			e.ExpiresAt.Format("2006-01-02 15:04:05"),
			strconv.FormatBool(e.Confirmed),
			e.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
}

func (s *Server) handleAddCredential(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
		slog.Error("Failed to create credential entry", "error", err)
		s.LogAuditRecord(c, "credential_add", "credential", entry.AgentID, "add credential failed", false, err)
		respondError(c, http.StatusInternalServerError, "failed to add credential")
		return
	}
	s.LogAuditRecord(c, "credential_add", "credential", entry.AgentID,
		"added credential (domain="+entry.Domain+", user="+entry.Username+", type="+entry.Type+")", true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":         "credential_update",
		"action":       "added",
		"id":           entry.ID,
		"agent_id":     entry.AgentID,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "id": entry.ID})
}

func (s *Server) handleGetCredential(c *gin.Context) {
	idStr := c.Param("cred_id")
	if _, err := strconv.ParseUint(idStr, 10, 32); err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	var cred db.CredentialEntry
	if !s.findOrFail(c, &cred, idStr, "credential") {
		return
	}
	cred.Notes = decryptCredNotes(cred.Notes)
	s.LogAuditRecord(c, "credential_view", "credential", cred.AgentID,
		"viewed credential id="+c.Param("cred_id")+" (domain="+cred.Domain+", user="+cred.Username+")", true, nil)
	respondSuccess(c, cred)
}

func (s *Server) handleDeleteCredential(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	idStr := c.Param("cred_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.Delete(&db.CredentialEntry{}, id).Error; err != nil {
		slog.Error("Failed to delete credential", "id", id, "error", err)
		s.LogAuditRecord(c, "credential_delete", "credential", "", "delete credential id="+idStr+" failed", false, err)
		respondError(c, http.StatusInternalServerError, "failed to delete")
		return
	}
	s.LogAuditRecord(c, "credential_delete", "credential", "", "deleted credential id="+idStr, true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":   "credential_update",
		"action": "deleted",
		"id":     id,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleUpdateCredential(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	idStr := c.Param("cred_id")
	if _, err := strconv.ParseUint(idStr, 10, 32); err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	var cred db.CredentialEntry
	if !s.findOrFail(c, &cred, idStr, "credential") {
		return
	}

	if tags := c.PostForm("tags"); tags != "" {
		cred.Tags = tags
	}
	if expiresAt := c.PostForm("expires_at"); expiresAt != "" {
		t, err := time.Parse("2006-01-02", expiresAt)
		if err == nil {
			cred.ExpiresAt = t
		}
	}
	if confirmed := c.PostForm("confirmed"); confirmed != "" {
		cred.Confirmed = confirmed == "true"
	}

	if err := s.db.Save(&cred).Error; err != nil {
		slog.Error("Failed to update credential", "id", cred.ID, "error", err)
		s.LogAuditRecord(c, "credential_update", "credential", cred.AgentID, "update credential id="+idStr+" failed", false, err)
		respondError(c, http.StatusInternalServerError, "failed to update")
		return
	}
	s.LogAuditRecord(c, "credential_update", "credential", cred.AgentID, "updated credential id="+idStr, true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":         "credential_update",
		"action":       "updated",
		"id":           cred.ID,
		"agent_id":     cred.AgentID,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleBatchAddTags(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		IDs  []uint   `json:"ids"`
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	if len(req.IDs) == 0 || len(req.Tags) == 0 {
		respondError(c, http.StatusBadRequest, "no ids or tags provided")
		return
	}

	newTags := strings.Join(req.Tags, ",")

	if err := s.db.Model(&db.CredentialEntry{}).
		Where("id IN ?", req.IDs).
		Update("tags", gorm.Expr("CASE WHEN tags = '' OR tags IS NULL THEN ? ELSE tags || ',' || ? END", newTags, newTags)).Error; err != nil {
		slog.Error("Failed to batch add tags", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to update tags")
		return
	}

	s.LogAuditRecord(c, "credential_tag", "credential", "", "added tags to "+strconv.Itoa(len(req.IDs))+" credentials", true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":   "credential_update",
		"action": "updated",
		"count":  len(req.IDs),
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "count": len(req.IDs)})
}

func (s *Server) handleToggleConfirmed(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	idStr := c.Param("cred_id")
	if _, err := strconv.ParseUint(idStr, 10, 32); err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	var cred db.CredentialEntry
	if !s.findOrFail(c, &cred, idStr, "credential") {
		return
	}

	cred.Confirmed = !cred.Confirmed

	if err := s.db.Save(&cred).Error; err != nil {
		slog.Error("Failed to toggle credential confirm", "id", cred.ID, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to update")
		return
	}
	s.LogAuditRecord(c, "credential_confirm", "credential", cred.AgentID,
		"set confirmed="+strconv.FormatBool(cred.Confirmed)+" for credential id="+idStr, true, nil)
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":         "credential_update",
		"action":       "updated",
		"id":           cred.ID,
		"agent_id":     cred.AgentID,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "confirmed": cred.Confirmed})
}
