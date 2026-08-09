package server

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
)

func newTestH2CRouter() http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/beacon", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.Data(http.StatusOK, "application/json", []byte("resp:"+string(body)))
	})
	r.GET("/api/v1/beacon", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte("get-ok"))
	})
	return r
}

func h2cTestClient() *http.Client {
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, tlsConfig *tls.Config) (net.Conn, error) {
			return net.DialTimeout(network, addr, 10*time.Second)
		},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}
}

func TestH2CBeaconListenerRoundTrip(t *testing.T) {
	l := NewH2CBeaconListener("127.0.0.1:0", newTestH2CRouter())
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	addr := l.Addr()
	if addr == "" {
		t.Fatal("listener addr empty")
	}

	body := `{"uuid":"aaaa-bbbb","seq":1}`
	resp, err := h2cTestClient().Post("http://"+addr+"/api/v1/beacon", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("h2c POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.ProtoMajor != 2 {
		t.Errorf("expected HTTP/2, got proto %d.%d", resp.ProtoMajor, resp.ProtoMinor)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if want := "resp:" + body; string(got) != want {
		t.Errorf("response mismatch: got %q want %q", string(got), want)
	}
}

func TestH2CBeaconListenerServesHTTP1Too(t *testing.T) {
	l := NewH2CBeaconListener("127.0.0.1:0", newTestH2CRouter())
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer l.Stop()

	resp, err := http.Get("http://" + l.Addr() + "/api/v1/beacon")
	if err != nil {
		t.Fatalf("HTTP/1.1 GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "get-ok" {
		t.Errorf("unexpected body %q", string(got))
	}
}

func TestH2CBeaconListenerStop(t *testing.T) {
	l := NewH2CBeaconListener("127.0.0.1:0", newTestH2CRouter())
	if err := l.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !l.IsRunning() {
		t.Fatal("expected running after start")
	}
	if err := l.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if l.IsRunning() {
		t.Fatal("expected stopped after Stop")
	}
	if _, err := h2cTestClient().Post("http://"+l.Addr()+"/api/v1/beacon", "application/json", strings.NewReader("{}")); err == nil {
		t.Fatal("expected connection to fail after stop")
	}
}
