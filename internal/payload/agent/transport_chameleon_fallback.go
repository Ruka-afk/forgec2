//go:build !chameleon

package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func utlsDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 15 * time.Second}
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

	tlsConn := tls.Client(rawConn, newAgentTLSConfig(serverName))
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func newUTLSTransport() *http.Transport {
	tr := &http.Transport{
		DisableKeepAlives: true,
		MaxIdleConns:      0,
	}
	tr.DialTLSContext = utlsDialContext
	return tr
}
