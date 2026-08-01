package server

import (
	"crypto/x509"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleGetCertInfo(c *gin.Context) {
	certPath := s.cfg.Server.CertFile
	if certPath == "" {
		certPath = "data/certs/server.crt"
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		respondError(c, http.StatusNotFound, "no certificate found")
		return
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		respondError(c, http.StatusInternalServerError, "failed to decode certificate")
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to parse certificate")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subject":      cert.Subject.CommonName,
		"issuer":       cert.Issuer.CommonName,
		"expires_at":   cert.NotAfter,
		"expires_in":   int(time.Until(cert.NotAfter).Hours() / 24),
		"dns_names":    cert.DNSNames,
		"ip_addresses": cert.IPAddresses,
		"is_self_signed": cert.Subject.CommonName == cert.Issuer.CommonName,
		"serial":       cert.SerialNumber.String(),
		"key_usage":    cert.KeyUsage,
	})
}

func (s *Server) handleRegenerateCert(c *gin.Context) {
	certPath := s.cfg.Server.CertFile
	keyPath := s.cfg.Server.KeyFile
	if certPath == "" {
		certPath = "data/certs/server.crt"
	}
	keyPath = filepath.Join(filepath.Dir(certPath), "server.key")

	// Remove existing files
	if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to remove existing cert", "path", certPath, "err", err)
	}
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to remove existing key", "path", keyPath, "err", err)
	}

	if err := crypto.GenerateSelfSignedCert(certPath, keyPath); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to regenerate certificate")
		return
	}

	s.LogAuditRecord(c, "regenerate_cert", "settings", "", "Regenerated self-signed certificate", true, nil)

	// Return new cert info
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "certificate regenerated"})
		return
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		c.JSON(http.StatusOK, gin.H{"status": "certificate regenerated"})
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "certificate regenerated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "certificate regenerated",
		"subject":      cert.Subject.CommonName,
		"issuer":       cert.Issuer.CommonName,
		"expires_at":   cert.NotAfter,
		"expires_in":   int(time.Until(cert.NotAfter).Hours() / 24),
		"dns_names":    cert.DNSNames,
		"ip_addresses": cert.IPAddresses,
		"is_self_signed": cert.Subject.CommonName == cert.Issuer.CommonName,
	})
}

func (s *Server) handleUploadCert(c *gin.Context) {
	certFile, _, err := c.Request.FormFile("cert")
	if err != nil {
		respondError(c, http.StatusBadRequest, "cert file required")
		return
	}
	defer certFile.Close()

	certData, err := io.ReadAll(io.LimitReader(certFile, 1<<20+1))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read cert file")
		return
	}
	if len(certData) > 1<<20 {
		respondError(c, http.StatusBadRequest, "cert file too large (max 1MB)")
		return
	}

	keyFile, _, err := c.Request.FormFile("key")
	if err != nil {
		respondError(c, http.StatusBadRequest, "key file required")
		return
	}
	defer keyFile.Close()

	keyData, err := io.ReadAll(io.LimitReader(keyFile, 1<<20+1))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read key file")
		return
	}
	if len(keyData) > 1<<20 {
		respondError(c, http.StatusBadRequest, "key file too large (max 1MB)")
		return
	}

	// Validate PEM format
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		respondError(c, http.StatusBadRequest, "invalid certificate PEM format")
		return
	}

	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil || (keyBlock.Type != "EC PRIVATE KEY" && keyBlock.Type != "RSA PRIVATE KEY") {
		respondError(c, http.StatusBadRequest, "invalid private key PEM format")
		return
	}

	certPath := s.cfg.Server.CertFile
	keyPath := s.cfg.Server.KeyFile
	if certPath == "" {
		certPath = "data/certs/server.crt"
	}
	keyPath = filepath.Join(filepath.Dir(certPath), "server.key")

	os.MkdirAll(filepath.Dir(certPath), 0700)

	if err := os.WriteFile(certPath, certData, 0600); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to write certificate")
		return
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to write private key")
		return
	}

	s.LogAuditRecord(c, "upload_cert", "settings", "", "Uploaded custom TLS certificate", true, nil)

	c.JSON(http.StatusOK, gin.H{"status": "certificate uploaded"})
}
