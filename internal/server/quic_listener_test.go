package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func testQUICTLS(t *testing.T) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h3", "fc2"},
	}
}

func TestQUICBeaconRoundTrip(t *testing.T) {
	tlsCfg := testQUICTLS(t)
	ln := NewQUICBeaconListener("127.0.0.1:0", tlsCfg)
	ln.SetHandler(func(_ string, req []byte) []byte {
		return append([]byte("echo:"), req...)
	})
	if err := ln.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3", "fc2"},
		MinVersion:         tls.VersionTLS13,
	}, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseWithError(0, "")
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if _, err := stream.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	got, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo:ping" {
		t.Fatalf("got %q", got)
	}
}
