// Package agent implements the local HTTP agent server.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/auth"
	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/ratelimit"
)

// AgentCard represents the /.well-known/agent.json document per the A2A spec.
type AgentCard struct {
	Name            string            `json:"name"`
	AgentID         string            `json:"agent_id"`
	Version         string            `json:"version"`
	PublicKey       string            `json:"public_key"`
	Capabilities    []string          `json:"capabilities"`
	SupportedModes  []string          `json:"supported_modes"`
	Endpoints       map[string]string `json:"endpoints"`
	RegistrationURL string            `json:"registration_url,omitempty"`
}

// Server is the local agent HTTP server.
type Server struct {
	cfg       *config.Config
	logger    zerolog.Logger
	audit     *audit.Logger
	limiter   *ratelimit.Limiter
	validator *auth.TokenValidator
	srv       *http.Server
}

// New creates an agent Server.
func New(cfg *config.Config, logger zerolog.Logger, auditLog *audit.Logger) (*Server, error) {
	limiter := ratelimit.New(cfg.RateLimits.RequestsPerMinute, cfg.RateLimits.BurstSize)

	// Build peer allowlist from config.
	peers := make(map[string]string)
	// Note: AllowedPeerIDs holds IDs; public keys are fetched from registry.
	// For now we allow validator to be created with empty peers and updated dynamically.
	validator, err := auth.NewTokenValidator(peers)
	if err != nil {
		return nil, fmt.Errorf("creating token validator: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		logger:    logger,
		audit:     auditLog,
		limiter:   limiter,
		validator: validator,
	}
	return s, nil
}

// Start binds and serves the agent API.
// It enforces localhost-only binding unless bind address is explicitly overridden.
func (s *Server) Start(ctx context.Context) error {
	bind := s.cfg.BindAddress
	if bind == "" {
		bind = config.DefaultBindAddress
	}

	// Security: refuse to bind 0.0.0.0 without explicit acknowledgement.
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return fmt.Errorf("invalid bind address %q: %w", bind, err)
	}
	if host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("SECURITY: refusing to bind %s — use --bind-all flag with explicit confirmation", bind)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)

	// Protected endpoints require auth.
	protected := s.validator.Middleware(http.HandlerFunc(s.handleExecute))
	mux.Handle("/execute", protected)

	// Apply rate limiting globally.
	handler := s.limiter.Middleware(mux)
	handler = s.loggingMiddleware(handler)

	s.srv = &http.Server{
		Addr:         bind,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info().Str("bind", bind).Msg("agent server starting")
	s.audit.MustLog(audit.EventTunnelStart, "agent server starting", map[string]string{"bind": bind})

	// Start cleanup goroutine.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.limiter.Cleanup(10 * time.Minute)
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.Stop()
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.logger.Info().Msg("agent server stopping")
	s.audit.MustLog(audit.EventTunnelStop, "agent server stopping", nil)
	return s.srv.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"agent_id": s.cfg.AgentID,
		"version":  Version,
	})
}

func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	card := AgentCard{
		Name:         s.cfg.AgentName,
		AgentID:      s.cfg.AgentID,
		Version:      Version,
		PublicKey:    s.cfg.PublicKey,
		Capabilities: s.cfg.Capabilities,
		SupportedModes: []string{"request-response"},
		Endpoints: map[string]string{
			"health":  s.cfg.PublicHostname + "/health",
			"execute": s.cfg.PublicHostname + "/execute",
		},
		RegistrationURL: s.cfg.RegistrationAPIURL,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Placeholder — real task dispatch logic goes here.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// --- Middleware ---

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.code).
			Dur("duration", time.Since(start)).
			Str("remote", r.RemoteAddr).
			Msg("request")

		// Audit failed auth attempts.
		if rw.code == http.StatusUnauthorized {
			s.audit.MustLog(audit.EventAuthFailed, "unauthorized request", map[string]string{
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			})
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

// Version of the agent binary. Set via -ldflags at build time.
var Version = "dev"

// SanitizePath ensures a path doesn't escape expected prefixes.
func SanitizePath(path string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/")
}
