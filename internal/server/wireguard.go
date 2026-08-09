package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"
)

type WireGuardPeer struct {
	PublicKey    string    `json:"public_key"`
	Endpoint     string    `json:"endpoint"`
	AllowedIPs   []string  `json:"allowed_ips"`
	PresharedKey string    `json:"preshared_key,omitempty"`
	LastHandshake time.Time `json:"last_handshake"`
	TransferRx   uint64    `json:"transfer_rx"`
	TransferTx   uint64    `json:"transfer_tx"`
	Connected    bool      `json:"connected"`
	AgentID      string    `json:"agent_id,omitempty"`
}

type WireGuardManager struct {
	mu          sync.RWMutex
	interfaceIP string
	listenPort  int
	privateKey  string
	publicKey   string
	peers       map[string]*WireGuardPeer
	subnet      string
	dnsServer   string
	stopCh      chan struct{}
}

func NewWireGuardManager(listenPort int, subnet string) (*WireGuardManager, error) {
	privKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	privKeyBytes[0] &= 0xf8
	privKeyBytes[31] = (privKeyBytes[31] & 0x7f) | 0x40

	pubKeyBytes, err := curve25519.X25519(privKeyBytes, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return &WireGuardManager{
		interfaceIP: subnet,
		listenPort:  listenPort,
		privateKey:  base64.StdEncoding.EncodeToString(privKeyBytes),
		publicKey:   base64.StdEncoding.EncodeToString(pubKeyBytes),
		peers:       make(map[string]*WireGuardPeer),
		subnet:      subnet,
		dnsServer:   "1.1.1.1",
		stopCh:      make(chan struct{}),
	}, nil
}

func (wg *WireGuardManager) AddPeer(publicKey, endpoint string, allowedIPs []string, agentID string) (*WireGuardPeer, error) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if _, exists := wg.peers[publicKey]; exists {
		return nil, fmt.Errorf("peer %s already exists", publicKey)
	}

	pskBytes := make([]byte, 32)
	rand.Read(pskBytes)

	peer := &WireGuardPeer{
		PublicKey:  publicKey,
		Endpoint:  endpoint,
		AllowedIPs: allowedIPs,
		PresharedKey: base64.StdEncoding.EncodeToString(pskBytes),
		Connected: false,
		AgentID:   agentID,
	}

	wg.peers[publicKey] = peer
	slog.Info("WireGuard peer added",
		"public_key", publicKey[:16]+"...",
		"endpoint", endpoint,
		"agent_id", agentID,
	)
	return peer, nil
}

func (wg *WireGuardManager) RemovePeer(publicKey string) {
	wg.mu.Lock()
	defer wg.mu.Unlock()
	delete(wg.peers, publicKey)
	slog.Info("WireGuard peer removed", "public_key", publicKey[:16]+"...")
}

func (wg *WireGuardManager) GetPeer(publicKey string) (*WireGuardPeer, bool) {
	wg.mu.RLock()
	defer wg.mu.RUnlock()
	p, ok := wg.peers[publicKey]
	return p, ok
}

func (wg *WireGuardManager) ListPeers() []*WireGuardPeer {
	wg.mu.RLock()
	defer wg.mu.RUnlock()
	result := make([]*WireGuardPeer, 0, len(wg.peers))
	for _, p := range wg.peers {
		result = append(result, p)
	}
	return result
}

func (wg *WireGuardManager) UpdatePeerStats(publicKey string, rx, tx uint64, handshake time.Time) {
	wg.mu.Lock()
	defer wg.mu.Unlock()
	if peer, ok := wg.peers[publicKey]; ok {
		peer.TransferRx = rx
		peer.TransferTx = tx
		peer.LastHandshake = handshake
		peer.Connected = time.Since(handshake) < 3*time.Minute
	}
}

func (wg *WireGuardManager) ConnectedPeerCount() int {
	wg.mu.RLock()
	defer wg.mu.RUnlock()
	count := 0
	for _, p := range wg.peers {
		if p.Connected {
			count++
		}
	}
	return count
}

func (wg *WireGuardManager) GeneratePeerConfig(publicKey string) (string, error) {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	peer, ok := wg.peers[publicKey]
	if !ok {
		return "", fmt.Errorf("peer not found")
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = <REDACTED>
Address = %s
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PresharedKey = <REDACTED>
PersistentKeepalive = 25
`, wg.subnet, wg.dnsServer, peer.PublicKey, peer.Endpoint, joinIPs(peer.AllowedIPs))
	return config, nil
}

func (wg *WireGuardManager) StartHealthCheck(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wg.stopCh:
				return
			case <-ticker.C:
				wg.healthCheck()
			}
		}
	}()
}

func (wg *WireGuardManager) healthCheck() {
	wg.mu.Lock()
	defer wg.mu.Unlock()
	for pk, peer := range wg.peers {
		wasConnected := peer.Connected
		peer.Connected = time.Since(peer.LastHandshake) < 3*time.Minute
		if wasConnected && !peer.Connected {
			slog.Warn("WireGuard peer disconnected",
				"public_key", pk[:16]+"...",
				"agent_id", peer.AgentID,
				"last_handshake", peer.LastHandshake,
			)
		}
	}
}

func joinIPs(ips []string) string {
	result := ""
	for i, ip := range ips {
		if i > 0 {
			result += ", "
		}
		result += ip
	}
	return result
}

func (wg *WireGuardManager) GetInterfaceConfig() string {
	_, cidr, _ := net.ParseCIDR(wg.subnet)
	var interfaceIP string
	if cidr != nil {
		interfaceIP = cidr.IP.String() + "/32"
	} else {
		interfaceIP = wg.interfaceIP
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = <SERVER_PRIVATE_KEY>
Address = %s
ListenPort = %d
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
`, interfaceIP, wg.listenPort)

	wg.mu.RLock()
	defer wg.mu.RUnlock()
	for _, peer := range wg.peers {
		if peer.Connected {
			config += fmt.Sprintf(`
[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
`, peer.PublicKey, peer.Endpoint, joinIPs(peer.AllowedIPs))
		}
	}
	return config
}
