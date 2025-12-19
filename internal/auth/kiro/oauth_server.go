package kiro

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// OAuthServer handles the local HTTP server for OAuth callbacks.
type OAuthServer struct {
	server     *http.Server
	port       int
	resultChan chan *OAuthResult
	errorChan  chan error
	mu         sync.Mutex
	running    bool
}

// OAuthResult contains the result of the OAuth callback.
type OAuthResult struct {
	Code  string
	State string
	Error string
}

// NewOAuthServer creates a new OAuth callback server.
func NewOAuthServer(port int) *OAuthServer {
	return &OAuthServer{
		port:       port,
		resultChan: make(chan *OAuthResult, 1),
		errorChan:  make(chan error, 1),
	}
}

// Start starts the OAuth callback server.
func (s *OAuthServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	if !s.isPortAvailable() {
		return fmt.Errorf("port %d is already in use", s.port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", s.handleCallback)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.running = true

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.errorChan <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop gracefully stops the OAuth callback server.
func (s *OAuthServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	s.running = false
	s.server = nil

	return err
}

// WaitForCallback waits for the OAuth callback with a timeout.
func (s *OAuthServer) WaitForCallback(timeout time.Duration) (*OAuthResult, error) {
	select {
	case result := <-s.resultChan:
		return result, nil
	case err := <-s.errorChan:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for OAuth callback")
	}
}

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	errorParam := query.Get("error")

	if errorParam != "" {
		s.sendResult(&OAuthResult{Error: errorParam})
		http.Error(w, fmt.Sprintf("OAuth error: %s", errorParam), http.StatusBadRequest)
		return
	}

	if code == "" {
		s.sendResult(&OAuthResult{Error: "no_code"})
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	s.sendResult(&OAuthResult{Code: code, State: state})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<html><body><h1>Authentication successful!</h1><p>You can close this window.</p></body></html>")
}

func (s *OAuthServer) sendResult(result *OAuthResult) {
	select {
	case s.resultChan <- result:
	default:
		log.Warn("OAuth result channel is full, result dropped")
	}
}

func (s *OAuthServer) isPortAvailable() bool {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
