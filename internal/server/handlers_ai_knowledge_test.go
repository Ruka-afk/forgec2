package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// multipartHeader builds a FileHeader for validateAIAttachment without a
// live HTTP request: the function only touches Filename, Size, Header and
// the opened stream.
func multipartHeader(t *testing.T, name string, data []byte, contentType string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	_, headers, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("parse form: %v", err)
	}
	if contentType != "" {
		headers.Header.Set("Content-Type", contentType)
	}
	return headers
}

// TestValidateAIAttachmentExtensionWhitelist proves only text extensions
// are accepted: a binary masquerading under a whitelisted name must fail
// content checks, and a binary extension is rejected outright.
func TestValidateAIAttachmentExtensionWhitelist(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"plain text ok", "notes.txt", false},
		{"markdown ok", "README.md", false},
		{"uppercase ext ok", "Notes.TXT", false},
		{"go source ok", "main.go", false},
		{"exe rejected", "payload.exe", true},
		{"dll rejected", "x.dll", true},
		{"wasm rejected", "module.wasm", true},
		{"png rejected", "img.png", true},
		{"no extension rejected", "Makefile", true},
		{"path traversal basename-stripped ok", "../../etc/passwd.txt", false},
		{"too long rejected", strings.Repeat("a", 300) + ".txt", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := multipartHeader(t, firstNonEmpty(strings.Repeat("a", 20), "ok.txt"), []byte("hello"), "text/plain")
			h.Filename = tc.filename
			h.Size = int64(len("hello"))
			_, _, err := validateAIAttachment(h)
			if tc.wantErr && err == nil {
				t.Fatalf("filename %q accepted, want rejection", tc.filename)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("filename %q rejected: %v", tc.filename, err)
			}
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return "x"
}

// TestValidateAIAttachmentRejectsBinary proves NUL bytes, invalid UTF-8 and
// a high control-character ratio are all refused: attachment content is
// concatenated into model prompts, so binary blobs must never pass.
func TestValidateAIAttachmentRejectsBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nul byte", []byte("hello\x00world")},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}},
		{"control heavy", bytes.Repeat([]byte{0x01}, 50)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := multipartHeader(t, "blob.txt", tc.data, "text/plain")
			h.Size = int64(len(tc.data))
			if _, _, err := validateAIAttachment(h); err == nil {
				t.Fatalf("binary content of %d bytes accepted", len(tc.data))
			}
		})
	}
}

// TestValidateAIAttachmentAcceptsText proves normal text (tabs included)
// passes, the media type falls back correctly, and content beyond the
// extract limit is truncated with valid UTF-8 preserved.
func TestValidateAIAttachmentAcceptsText(t *testing.T) {
	text := "line one\nline two\ttab\n"
	h := multipartHeader(t, "a.txt", []byte(text), "")
	h.Size = int64(len(text))
	data, mediaType, err := validateAIAttachment(h)
	if err != nil {
		t.Fatalf("plain text rejected: %v", err)
	}
	if string(data) != text {
		t.Fatalf("data = %q, want the original", data)
	}
	if mediaType != "text/plain; charset=utf-8" {
		t.Fatalf("mediaType = %q, want the text/plain fallback for an empty header", mediaType)
	}

	// An explicit content type is preserved.
	h2 := multipartHeader(t, "a.csv", []byte("a,b\n"), "text/csv")
	data2, mt2, err := validateAIAttachment(h2)
	if err != nil || mt2 != "text/csv" || string(data2) != "a,b\n" {
		t.Fatalf("csv: data=%q mt=%q err=%v", data2, mt2, err)
	}

	// octet-stream is treated as absent (fallback to text/plain).
	h3 := multipartHeader(t, "a.md", []byte("# hi\n"), "application/octet-stream")
	_, mt3, err := validateAIAttachment(h3)
	if err != nil || mt3 != "text/plain; charset=utf-8" {
		t.Fatalf("octet-stream fallback: mt=%q err=%v", mt3, err)
	}

	// Oversized content truncates to the extract limit on a rune boundary.
	big := strings.Repeat("a", aiAttachmentExtractBytes+10)
	h4 := multipartHeader(t, "big.txt", []byte(big), "text/plain")
	h4.Size = int64(len(big))
	data4, _, err := validateAIAttachment(h4)
	if err != nil {
		t.Fatalf("oversized text rejected: %v", err)
	}
	if len(data4) != aiAttachmentExtractBytes {
		t.Fatalf("extracted %d bytes, want the %d-byte cap", len(data4), aiAttachmentExtractBytes)
	}
	if !utf8.Valid(data4) {
		t.Fatal("truncation produced invalid UTF-8")
	}
}

// TestValidateAIAttachmentSizeFloor proves empty uploads are refused.
func TestValidateAIAttachmentSizeFloor(t *testing.T) {
	h := multipartHeader(t, "empty.txt", nil, "text/plain")
	h.Size = 0
	if _, _, err := validateAIAttachment(h); err == nil {
		t.Fatal("zero-byte attachment accepted")
	}
}

// ── Session / collection authorization ─────────────────────────────────────

func newAIKnowledgeTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	// SetupTestDB's AutoMigrate list predates the knowledge tables.
	if err := database.AutoMigrate(
		&db.AIAttachment{}, &db.AIKnowledgeCollection{},
		&db.AIKnowledgeSource{}, &db.AIKnowledgeChunk{},
	); err != nil {
		t.Fatalf("migrate knowledge tables: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Crypto.LootKey = strings.Repeat("cd", 32)
	return &Server{db: database, cfg: cfg}
}

func aiCtx(username, role string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user", username)
	c.Set("user_role", role)
	return c
}

// TestAuthorizedAISessionCrossTenant404 proves a session is only visible to
// its owner in the same tenant: a foreign id must 404, not leak.
func TestAuthorizedAISessionCrossTenant404(t *testing.T) {
	s := newAIKnowledgeTestServer(t)
	owner := db.User{Username: "ai-owner", TenantID: 1, Role: "operator", IsActive: true}
	if err := s.db.Create(&owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	other := db.User{Username: "ai-attacker", TenantID: 2, Role: "operator", IsActive: true}
	if err := s.db.Create(&other).Error; err != nil {
		t.Fatalf("seed attacker: %v", err)
	}
	sess := db.AIChatSession{OwnerID: owner.ID, TenantID: 1, Title: "t"}
	if err := s.db.Create(&sess).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}

	c := aiCtx("ai-attacker", "operator")
	c.Params = gin.Params{{Key: "id", Value: uintToStr(sess.ID)}}
	if _, ok := s.authorizedAISession(c, aiPrincipal{UserID: other.ID, TenantID: 2}); ok {
		t.Fatal("cross-tenant session was authorized")
	}

	// Owner in the right tenant succeeds.
	c2 := aiCtx("ai-owner", "operator")
	c2.Params = gin.Params{{Key: "id", Value: uintToStr(sess.ID)}}
	if _, ok := s.authorizedAISession(c2, aiPrincipal{UserID: owner.ID, TenantID: 1}); !ok {
		t.Fatal("owner's own session was refused")
	}
}

// TestFindWritableAICollectionPermissionMatrix proves the collection gate:
// owner may write; a different user without the manage permission cannot;
// shared collections require the permission even for members.
func TestFindWritableAICollectionPermissionMatrix(t *testing.T) {
	s := newAIKnowledgeTestServer(t)
	owner := db.User{Username: "kn-owner", TenantID: 1, Role: "operator", IsActive: true}
	if err := s.db.Create(&owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	member := db.User{Username: "kn-member", TenantID: 1, Role: "viewer", IsActive: true}
	if err := s.db.Create(&member).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	admin := db.User{Username: "kn-admin", TenantID: 1, Role: "admin", IsActive: true}
	if err := s.db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	private := db.AIKnowledgeCollection{TenantID: 1, OwnerID: owner.ID, Name: "priv", Shared: false}
	if err := s.db.Create(&private).Error; err != nil {
		t.Fatalf("seed private: %v", err)
	}
	shared := db.AIKnowledgeCollection{TenantID: 1, OwnerID: owner.ID, Name: "sh", Shared: true}
	if err := s.db.Create(&shared).Error; err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	ownerP := aiPrincipal{UserID: owner.ID, Username: "kn-owner", TenantID: 1, Role: "operator"}
	memberP := aiPrincipal{UserID: member.ID, Username: "kn-member", TenantID: 1, Role: "viewer"}
	adminP := aiPrincipal{UserID: admin.ID, Username: "kn-admin", TenantID: 1, Role: "admin"}

	mkCtx := func(coll db.AIKnowledgeCollection) *gin.Context {
		c := aiCtx("kn-op", "operator")
		c.Params = gin.Params{{Key: "collectionID", Value: uintToStr(coll.ID)}}
		return c
	}

	// Owner writes a private collection.
	if _, ok := s.findWritableAICollection(mkCtx(private), ownerP); !ok {
		t.Fatal("owner refused on their own private collection")
	}
	// A shared collection requires the manage permission even for its own
	// owner — sharing is a policy act, not a convenience.
	if _, ok := s.findWritableAICollection(mkCtx(shared), ownerP); ok {
		t.Fatal("owner wrote a shared collection without the manage permission")
	}
	// A plain member cannot write the owner's private collection.
	if _, ok := s.findWritableAICollection(mkCtx(private), memberP); ok {
		t.Fatal("member wrote the owner's private collection")
	}
	// A plain member cannot write a shared collection either (needs manage).
	if _, ok := s.findWritableAICollection(mkCtx(shared), memberP); ok {
		t.Fatal("member wrote a shared collection without the manage permission")
	}
	// An admin (manage permission) can write the shared collection.
	if _, ok := s.findWritableAICollection(mkCtx(shared), adminP); !ok {
		t.Fatal("admin refused on the shared collection")
	}
	// An admin can also write the private one (manage covers all).
	if _, ok := s.findWritableAICollection(mkCtx(private), adminP); !ok {
		t.Fatal("admin refused on a private collection")
	}
	// A collection from another tenant is invisible.
	foreign := db.AIKnowledgeCollection{TenantID: 9, OwnerID: 99, Name: "f", Shared: true}
	if err := s.db.Create(&foreign).Error; err != nil {
		t.Fatalf("seed foreign: %v", err)
	}
	if _, ok := s.findWritableAICollection(mkCtx(foreign), ownerP); ok {
		t.Fatal("cross-tenant collection authorized")
	}
}

// ── Chunking / search terms / blind index ───────────────────────────────────

// TestSplitAIKnowledgeChunkOverlap proves chunking honors the rune window
// and overlap and always terminates, and empty content yields nothing.
func TestSplitAIKnowledgeChunkOverlap(t *testing.T) {
	if got := splitAIKnowledge(""); len(got) != 0 {
		t.Fatalf("empty content produced %d chunks", len(got))
	}

	short := "hello world"
	got := splitAIKnowledge(short)
	if len(got) != 1 || got[0] != short {
		t.Fatalf("short content = %v, want a single unchanged chunk", got)
	}

	long := strings.Repeat("x", aiKnowledgeChunkRunes*3+5)
	chunks := splitAIKnowledge(long)
	if len(chunks) < 3 {
		t.Fatalf("long content produced %d chunks, want >= 3", len(chunks))
	}
	// Consecutive chunks must overlap by exactly the configured amount so
	// no result is split across a boundary without context.
	for i := 1; i < len(chunks); i++ {
		prev := []rune(chunks[i-1])
		cur := []rune(chunks[i])
		if len(prev) < aiKnowledgeChunkOverlap {
			continue
		}
		if string(prev[len(prev)-aiKnowledgeChunkOverlap:]) != string(cur[:aiKnowledgeChunkOverlap]) {
			t.Fatalf("chunk %d does not overlap its predecessor by %d runes", i, aiKnowledgeChunkOverlap)
		}
	}
}

// TestAISearchTermsCJKAndLatin proves the term extractor handles unicode
// words, CJK bigrams and punctuation, and caps the result set.
func TestAISearchTermsCJKAndLatin(t *testing.T) {
	terms := aiSearchTerms("hello world_cache test-node 你好世界 hello hello")
	joined := strings.Join(terms, ",")
	for _, want := range []string{"hello", "world_cache", "test-node"} {
		if !contains(joined, want) {
			t.Fatalf("terms %q missing %q", joined, want)
		}
	}
	// CJK runs become overlapping bigrams.
	if !contains(joined, "你好") || !contains(joined, "世界") {
		t.Fatalf("CJK bigrams missing: %q", joined)
	}
	// Duplicates collapse.
	count := 0
	for _, term := range terms {
		if term == "hello" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate term appeared %d times", count)
	}
	// Single characters are never terms.
	for _, term := range terms {
		if len([]rune(term)) < 2 {
			t.Fatalf("single-rune term %q leaked", term)
		}
	}
	// Output is sorted.
	for i := 1; i < len(terms); i++ {
		if terms[i-1] > terms[i] {
			t.Fatalf("terms not sorted: %q", terms)
		}
	}
}

// TestBlindAIKnowledgeTokensAreDeterministic proves the blind index is
// keyed by the loot key (stable across runs, opaque without it, and
// never containing the plaintext term).
func TestBlindAIKnowledgeTokensAreDeterministic(t *testing.T) {
	s := newAIKnowledgeTestServer(t)
	a := s.blindAIKnowledgeTokens("secret-project alpha")
	b := s.blindAIKnowledgeTokens("secret-project alpha")
	if a != b {
		t.Fatalf("blind tokens are not deterministic:\n%s\n%s", a, b)
	}
	if strings.Contains(a, "secret-project") {
		t.Fatal("blind index leaked the plaintext term")
	}
	// A different key must produce a different index.
	s.cfg.Crypto.LootKey = strings.Repeat("ef", 32)
	c := s.blindAIKnowledgeTokens("secret-project alpha")
	if c == a {
		t.Fatal("blind tokens identical under a different key")
	}
}
