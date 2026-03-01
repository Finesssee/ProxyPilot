package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

// --- Mock implementations ---

// mockStore implements Store interface for testing.
type mockStore struct {
	mu      sync.Mutex
	auths   map[string]*Auth
	listErr error
	saveErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		auths: make(map[string]*Auth),
	}
}

func (s *mockStore) List(ctx context.Context) ([]*Auth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]*Auth, 0, len(s.auths))
	for _, auth := range s.auths {
		result = append(result, auth.Clone())
	}
	return result, nil
}

func (s *mockStore) Save(ctx context.Context, auth *Auth) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return "", s.saveErr
	}
	s.auths[auth.ID] = auth.Clone()
	return auth.ID, nil
}

func (s *mockStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.auths, id)
	return nil
}

func (s *mockStore) get(id string) *Auth {
	s.mu.Lock()
	defer s.mu.Unlock()
	if auth, ok := s.auths[id]; ok {
		return auth.Clone()
	}
	return nil
}

// mockExecutor implements ProviderExecutor interface for testing.
type mockExecutor struct {
	provider      string
	executeFunc   func(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error)
	executeCount  int
	mu            sync.Mutex
	lastAuth      *Auth
	lastRequest   cliproxyexecutor.Request
	refreshFunc   func(ctx context.Context, auth *Auth) (*Auth, error)
	countTokensFn func(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error)
	embedFunc     func(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error)
}

func (e *mockExecutor) Identifier() string {
	return e.provider
}

func (e *mockExecutor) Execute(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	e.mu.Lock()
	e.executeCount++
	e.lastAuth = auth.Clone()
	e.lastRequest = req
	e.mu.Unlock()
	if e.executeFunc != nil {
		return e.executeFunc(ctx, auth, req, opts)
	}
	return cliproxyexecutor.Response{Payload: []byte(`{"result":"ok"}`)}, nil
}

func (e *mockExecutor) ExecuteStream(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	ch := make(chan cliproxyexecutor.StreamChunk, 1)
	ch <- cliproxyexecutor.StreamChunk{Payload: []byte(`{"stream":"ok"}`)}
	close(ch)
	return &cliproxyexecutor.StreamResult{Chunks: ch}, nil
}

func (e *mockExecutor) Refresh(ctx context.Context, auth *Auth) (*Auth, error) {
	if e.refreshFunc != nil {
		return e.refreshFunc(ctx, auth)
	}
	return auth.Clone(), nil
}

func (e *mockExecutor) CountTokens(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	if e.countTokensFn != nil {
		return e.countTokensFn(ctx, auth, req, opts)
	}
	return cliproxyexecutor.Response{Payload: []byte(`{"tokens":100}`)}, nil
}

func (e *mockExecutor) Embed(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	if e.embedFunc != nil {
		return e.embedFunc(ctx, auth, req, opts)
	}
	return cliproxyexecutor.Response{Payload: []byte(`{"embeddings":[]}`)}, nil
}

func (e *mockExecutor) HttpRequest(_ context.Context, _ *Auth, _ *http.Request) (*http.Response, error) {
	return nil, errors.New("mock executor: HttpRequest not implemented")
}

func (e *mockExecutor) getExecuteCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executeCount
}

// mockSelector implements Selector interface for testing.
type mockSelector struct {
	pickFunc func(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error)
}

func (s *mockSelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	if s.pickFunc != nil {
		return s.pickFunc(ctx, provider, model, opts, auths)
	}
	if len(auths) == 0 {
		return nil, &Error{Code: "no_auth", Message: "no auths available"}
	}
	return auths[0], nil
}

// mockHook implements Hook interface for testing.
type mockHook struct {
	mu               sync.Mutex
	registeredAuths  []*Auth
	updatedAuths     []*Auth
	results          []Result
	onRegisteredFunc func(ctx context.Context, auth *Auth)
	onUpdatedFunc    func(ctx context.Context, auth *Auth)
	onResultFunc     func(ctx context.Context, result Result)
}

func (h *mockHook) OnAuthRegistered(ctx context.Context, auth *Auth) {
	h.mu.Lock()
	h.registeredAuths = append(h.registeredAuths, auth.Clone())
	h.mu.Unlock()
	if h.onRegisteredFunc != nil {
		h.onRegisteredFunc(ctx, auth)
	}
}

func (h *mockHook) OnAuthUpdated(ctx context.Context, auth *Auth) {
	h.mu.Lock()
	h.updatedAuths = append(h.updatedAuths, auth.Clone())
	h.mu.Unlock()
	if h.onUpdatedFunc != nil {
		h.onUpdatedFunc(ctx, auth)
	}
}

func (h *mockHook) OnResult(ctx context.Context, result Result) {
	h.mu.Lock()
	h.results = append(h.results, result)
	h.mu.Unlock()
	if h.onResultFunc != nil {
		h.onResultFunc(ctx, result)
	}
}

func (h *mockHook) getRegisteredAuths() []*Auth {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]*Auth, len(h.registeredAuths))
	copy(result, h.registeredAuths)
	return result
}

func (h *mockHook) getResults() []Result {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]Result, len(h.results))
	copy(result, h.results)
	return result
}

// statusError implements cliproxyexecutor.StatusError for testing.
type statusError struct {
	statusCode int
	message    string
	retryAfter *time.Duration
}

func (e *statusError) Error() string {
	return e.message
}

func (e *statusError) StatusCode() int {
	return e.statusCode
}

func (e *statusError) RetryAfter() *time.Duration {
	return e.retryAfter
}

// --- Test cases ---

func TestManager_NewManager_DefaultSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		selector     Selector
		hook         Hook
		wantRRPicker bool
		wantNoopHook bool
	}{
		{
			name:         "nil selector uses RoundRobinSelector",
			selector:     nil,
			hook:         &mockHook{},
			wantRRPicker: true,
			wantNoopHook: false,
		},
		{
			name:         "nil hook uses NoopHook",
			selector:     &mockSelector{},
			hook:         nil,
			wantRRPicker: false,
			wantNoopHook: true,
		},
		{
			name:         "both nil uses defaults",
			selector:     nil,
			hook:         nil,
			wantRRPicker: true,
			wantNoopHook: true,
		},
		{
			name:         "custom selector and hook used as-is",
			selector:     &mockSelector{},
			hook:         &mockHook{},
			wantRRPicker: false,
			wantNoopHook: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, tt.selector, tt.hook)

			if m == nil {
				t.Fatal("NewManager returned nil")
			}
			if m.auths == nil {
				t.Error("NewManager did not initialize auths map")
			}
			if m.executors == nil {
				t.Error("NewManager did not initialize executors map")
			}
			if m.providerOffsets == nil {
				t.Error("NewManager did not initialize providerOffsets map")
			}

			if tt.wantRRPicker {
				if _, ok := m.selector.(*RoundRobinSelector); !ok {
					t.Errorf("expected RoundRobinSelector when selector is nil, got %T", m.selector)
				}
			}
			if tt.wantNoopHook {
				if _, ok := m.hook.(NoopHook); !ok {
					t.Errorf("expected NoopHook when hook is nil, got %T", m.hook)
				}
			}
		})
	}
}

func TestManager_Register_GeneratesID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		auth      *Auth
		wantNewID bool
		wantNil   bool
	}{
		{
			name:      "nil auth returns nil",
			auth:      nil,
			wantNewID: false,
			wantNil:   true,
		},
		{
			name:      "empty ID gets generated UUID",
			auth:      &Auth{Provider: "test"},
			wantNewID: true,
			wantNil:   false,
		},
		{
			name:      "existing ID is preserved",
			auth:      &Auth{ID: "custom-id", Provider: "test"},
			wantNewID: false,
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(newMockStore(), nil, nil)
			ctx := context.Background()

			result, err := m.Register(ctx, tt.auth)

			if err != nil {
				t.Fatalf("Register() unexpected error: %v", err)
			}

			if tt.wantNil {
				if result != nil {
					t.Errorf("Register() expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Register() returned nil unexpectedly")
			}

			if tt.wantNewID {
				if result.ID == "" {
					t.Error("Register() did not generate ID for auth with empty ID")
				}
				// UUID should be 36 characters (8-4-4-4-12 format)
				if len(result.ID) != 36 {
					t.Errorf("Register() generated ID %q does not look like UUID", result.ID)
				}
			} else if tt.auth != nil && tt.auth.ID != "" {
				if result.ID != tt.auth.ID {
					t.Errorf("Register() changed existing ID from %q to %q", tt.auth.ID, result.ID)
				}
			}
		})
	}
}

func TestManager_Register_PersistsToStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		auth        *Auth
		wantInStore bool
	}{
		{
			name: "auth with metadata is persisted",
			auth: &Auth{
				ID:       "test-id",
				Provider: "test",
				Metadata: map[string]any{"key": "value"},
			},
			wantInStore: true,
		},
		{
			name: "auth without metadata is not persisted",
			auth: &Auth{
				ID:       "test-id-no-meta",
				Provider: "test",
				Metadata: nil,
			},
			wantInStore: false,
		},
		{
			name: "runtime_only auth is not persisted",
			auth: &Auth{
				ID:         "runtime-only-id",
				Provider:   "test",
				Metadata:   map[string]any{"key": "value"},
				Attributes: map[string]string{"runtime_only": "true"},
			},
			wantInStore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore()
			m := NewManager(store, nil, nil)
			ctx := context.Background()

			_, err := m.Register(ctx, tt.auth)
			if err != nil {
				t.Fatalf("Register() error: %v", err)
			}

			stored := store.get(tt.auth.ID)

			if tt.wantInStore {
				if stored == nil {
					t.Error("Register() did not persist auth to store")
				} else if stored.ID != tt.auth.ID {
					t.Errorf("stored auth ID = %q, want %q", stored.ID, tt.auth.ID)
				}
			} else {
				if stored != nil {
					t.Error("Register() persisted auth that should not be persisted")
				}
			}
		})
	}
}

func TestManager_Execute_NoProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []string
		wantCode  string
	}{
		{
			name:      "nil providers",
			providers: nil,
			wantCode:  "provider_not_found",
		},
		{
			name:      "empty providers slice",
			providers: []string{},
			wantCode:  "provider_not_found",
		},
		{
			name:      "providers with only whitespace",
			providers: []string{"  ", "\t"},
			wantCode:  "provider_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)
			ctx := context.Background()
			req := cliproxyexecutor.Request{Model: "test-model"}

			_, err := m.Execute(ctx, tt.providers, req, cliproxyexecutor.Options{})

			if err == nil {
				t.Fatal("Execute() expected error, got nil")
			}

			var authErr *Error
			if !errors.As(err, &authErr) {
				t.Fatalf("Execute() error type = %T, want *Error", err)
			}
			if authErr.Code != tt.wantCode {
				t.Errorf("Execute() error code = %q, want %q", authErr.Code, tt.wantCode)
			}
		})
	}
}

func TestManager_Execute_SuccessfulRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providers    []string
		model        string
		authProvider string
		wantPayload  string
	}{
		{
			name:         "single provider success",
			providers:    []string{"testprovider"},
			model:        "test-model",
			authProvider: "testprovider",
			wantPayload:  `{"result":"success"}`,
		},
		{
			name:         "provider with mixed case normalized",
			providers:    []string{"TestProvider"},
			model:        "test-model",
			authProvider: "testprovider",
			wantPayload:  `{"result":"success"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore()
			hook := &mockHook{}
			m := NewManager(store, nil, hook)
			ctx := context.Background()

			// Register an executor
			executor := &mockExecutor{
				provider: tt.authProvider,
				executeFunc: func(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
					return cliproxyexecutor.Response{Payload: []byte(tt.wantPayload)}, nil
				},
			}
			m.RegisterExecutor(executor)

			// Register an auth
			auth := &Auth{
				ID:       "test-auth",
				Provider: tt.authProvider,
				Metadata: map[string]any{"test": true},
			}
			_, _ = m.Register(ctx, auth)

			// Register the test model with the client in the global registry
			if tt.model != "" {
				registry.GetGlobalRegistry().RegisterClient(auth.ID, tt.authProvider, []*registry.ModelInfo{
					{ID: tt.model},
				})
			}

			req := cliproxyexecutor.Request{Model: tt.model}
			resp, err := m.Execute(ctx, tt.providers, req, cliproxyexecutor.Options{})

			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if string(resp.Payload) != tt.wantPayload {
				t.Errorf("Execute() payload = %q, want %q", string(resp.Payload), tt.wantPayload)
			}

			// Verify hook was called with success result
			results := hook.getResults()
			if len(results) == 0 {
				t.Error("Execute() did not trigger OnResult hook")
			} else if !results[len(results)-1].Success {
				t.Error("Execute() result should indicate success")
			}
		})
	}
}

func TestManager_Execute_RetryOnFailure(t *testing.T) {
	// Not parallel due to global quotaCooldownDisabled state

	tests := []struct {
		name           string
		numAuths       int
		failFirstN     int
		wantSuccess    bool
		wantExecutions int
	}{
		{
			name:           "fails once then succeeds with second auth",
			numAuths:       2,
			failFirstN:     1,
			wantSuccess:    true,
			wantExecutions: 2,
		},
		{
			name:           "fails all auths",
			numAuths:       2,
			failFirstN:     2,
			wantSuccess:    false,
			wantExecutions: 2,
		},
		{
			name:           "succeeds on first try",
			numAuths:       3,
			failFirstN:     0,
			wantSuccess:    true,
			wantExecutions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disable quota cooldown for this test to allow immediate retries
			SetQuotaCooldownDisabled(true)

			m := NewManager(nil, nil, nil)
			ctx := context.Background()

			// Track execution count
			var execCount int
			var mu sync.Mutex

			executor := &mockExecutor{
				provider: "testprovider",
				executeFunc: func(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
					mu.Lock()
					execCount++
					current := execCount
					mu.Unlock()

					if current <= tt.failFirstN {
						return cliproxyexecutor.Response{}, &statusError{
							statusCode: 500,
							message:    "server error",
						}
					}
					return cliproxyexecutor.Response{Payload: []byte(`{"ok":true}`)}, nil
				},
			}
			m.RegisterExecutor(executor)

			// Register multiple auths
			for i := 0; i < tt.numAuths; i++ {
				auth := &Auth{
					ID:       "auth-" + string(rune('a'+i)),
					Provider: "testprovider",
				}
				m.mu.Lock()
				m.auths[auth.ID] = auth
				m.mu.Unlock()
				// Register the test model with each client
				registry.GetGlobalRegistry().RegisterClient(auth.ID, "testprovider", []*registry.ModelInfo{
					{ID: "test-model"},
				})
			}

			req := cliproxyexecutor.Request{Model: "test-model"}
			_, err := m.Execute(ctx, []string{"testprovider"}, req, cliproxyexecutor.Options{})

			mu.Lock()
			finalCount := execCount
			mu.Unlock()

			if tt.wantSuccess {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
			}

			if finalCount != tt.wantExecutions {
				t.Errorf("Execute() execution count = %d, want %d", finalCount, tt.wantExecutions)
			}
		})
	}
}

func TestManager_MarkResult_Success_ClearsQuota(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initialQuota      QuotaState
		model             string
		wantQuotaExceeded bool
		wantModelUnavail  bool
		wantStatusActive  bool
	}{
		{
			name: "success clears quota exceeded on model",
			initialQuota: QuotaState{
				Exceeded:     true,
				Reason:       "quota",
				BackoffLevel: 3,
			},
			model:             "test-model",
			wantQuotaExceeded: false,
			wantModelUnavail:  false,
			wantStatusActive:  true,
		},
		{
			name: "success clears model unavailable state",
			initialQuota: QuotaState{
				Exceeded:      true,
				Reason:        "rate_limit",
				NextRecoverAt: time.Now().Add(time.Hour),
			},
			model:             "test-model",
			wantQuotaExceeded: false,
			wantModelUnavail:  false,
			wantStatusActive:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)
			ctx := context.Background()

			// Create auth with initial quota state
			auth := &Auth{
				ID:       "test-auth",
				Provider: "testprovider",
				Status:   StatusError,
				ModelStates: map[string]*ModelState{
					tt.model: {
						Status:      StatusError,
						Unavailable: true,
						Quota:       tt.initialQuota,
					},
				},
			}
			m.mu.Lock()
			m.auths[auth.ID] = auth
			m.mu.Unlock()

			// Mark successful result
			result := Result{
				AuthID:   auth.ID,
				Provider: "testprovider",
				Model:    tt.model,
				Success:  true,
			}
			m.MarkResult(ctx, result)

			// Verify state was cleared
			m.mu.RLock()
			updated := m.auths[auth.ID]
			m.mu.RUnlock()

			if updated == nil {
				t.Fatal("auth not found after MarkResult")
			}

			modelState := updated.ModelStates[tt.model]
			if modelState == nil {
				t.Fatal("model state not found after MarkResult")
			}

			if modelState.Quota.Exceeded != tt.wantQuotaExceeded {
				t.Errorf("model Quota.Exceeded = %v, want %v", modelState.Quota.Exceeded, tt.wantQuotaExceeded)
			}
			if modelState.Unavailable != tt.wantModelUnavail {
				t.Errorf("model Unavailable = %v, want %v", modelState.Unavailable, tt.wantModelUnavail)
			}
			if tt.wantStatusActive && modelState.Status != StatusActive {
				t.Errorf("model Status = %v, want %v", modelState.Status, StatusActive)
			}
		})
	}
}

func TestManager_MarkResult_Failure_429_QuotaBackoff(t *testing.T) {
	// Not parallel due to global quotaCooldownDisabled state

	tests := []struct {
		name                string
		initialBackoffLevel int
		retryAfter          *time.Duration
		wantQuotaExceeded   bool
		wantBackoffIncr     bool
	}{
		{
			name:                "429 without retry-after increments backoff",
			initialBackoffLevel: 0,
			retryAfter:          nil,
			wantQuotaExceeded:   true,
			wantBackoffIncr:     true,
		},
		{
			name:                "429 with retry-after uses provided duration",
			initialBackoffLevel: 0,
			retryAfter:          durationPtr(30 * time.Second),
			wantQuotaExceeded:   true,
			wantBackoffIncr:     false,
		},
		{
			name:                "repeated 429 continues backoff",
			initialBackoffLevel: 2,
			retryAfter:          nil,
			wantQuotaExceeded:   true,
			wantBackoffIncr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure quota cooldown is enabled for this test
			SetQuotaCooldownDisabled(false)

			m := NewManager(nil, nil, nil)
			ctx := context.Background()
			model := "test-model"

			// Create auth with initial state
			auth := &Auth{
				ID:       "test-auth",
				Provider: "testprovider",
				ModelStates: map[string]*ModelState{
					model: {
						Status: StatusActive,
						Quota: QuotaState{
							BackoffLevel: tt.initialBackoffLevel,
						},
					},
				},
			}
			m.mu.Lock()
			m.auths[auth.ID] = auth
			m.mu.Unlock()

			// Mark 429 failure
			result := Result{
				AuthID:     auth.ID,
				Provider:   "testprovider",
				Model:      model,
				Success:    false,
				RetryAfter: tt.retryAfter,
				Error: &Error{
					HTTPStatus: 429,
					Message:    "rate limited",
				},
			}
			m.MarkResult(ctx, result)

			// Verify quota state
			m.mu.RLock()
			updated := m.auths[auth.ID]
			m.mu.RUnlock()

			if updated == nil {
				t.Fatal("auth not found after MarkResult")
			}

			modelState := updated.ModelStates[model]
			if modelState == nil {
				t.Fatal("model state not found after MarkResult")
			}

			if modelState.Quota.Exceeded != tt.wantQuotaExceeded {
				t.Errorf("Quota.Exceeded = %v, want %v", modelState.Quota.Exceeded, tt.wantQuotaExceeded)
			}

			if tt.wantBackoffIncr {
				if modelState.Quota.BackoffLevel <= tt.initialBackoffLevel {
					t.Errorf("BackoffLevel = %d, want > %d", modelState.Quota.BackoffLevel, tt.initialBackoffLevel)
				}
			}

			if modelState.Quota.NextRecoverAt.IsZero() {
				t.Error("NextRecoverAt should be set after 429")
			}
		})
	}
}

func TestManager_GetByID_ExistingAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authID     string
		authExists bool
		wantFound  bool
	}{
		{
			name:       "existing auth is found",
			authID:     "existing-auth",
			authExists: true,
			wantFound:  true,
		},
		{
			name:       "auth with attributes is cloned properly",
			authID:     "auth-with-attrs",
			authExists: true,
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)

			if tt.authExists {
				auth := &Auth{
					ID:         tt.authID,
					Provider:   "testprovider",
					Attributes: map[string]string{"key": "value"},
					Metadata:   map[string]any{"meta": "data"},
				}
				m.mu.Lock()
				m.auths[tt.authID] = auth
				m.mu.Unlock()
			}

			got, found := m.GetByID(tt.authID)

			if found != tt.wantFound {
				t.Errorf("GetByID() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if got == nil {
					t.Fatal("GetByID() returned nil for existing auth")
				}
				if got.ID != tt.authID {
					t.Errorf("GetByID() auth.ID = %q, want %q", got.ID, tt.authID)
				}

				// Verify it's a clone, not the original
				m.mu.RLock()
				original := m.auths[tt.authID]
				m.mu.RUnlock()

				if got == original {
					t.Error("GetByID() should return a clone, not the original pointer")
				}

				// Modify returned auth and verify original is unchanged
				got.Attributes["modified"] = "true"
				if original.Attributes["modified"] == "true" {
					t.Error("modifying returned auth affected the original")
				}
			}
		})
	}
}

func TestManager_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		authID string
	}{
		{
			name:   "empty ID returns not found",
			authID: "",
		},
		{
			name:   "non-existent ID returns not found",
			authID: "does-not-exist",
		},
		{
			name:   "whitespace ID returns not found",
			authID: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)

			// Add some auths to make sure we're not just failing on empty map
			m.mu.Lock()
			m.auths["other-auth"] = &Auth{ID: "other-auth", Provider: "test"}
			m.mu.Unlock()

			got, found := m.GetByID(tt.authID)

			if found {
				t.Errorf("GetByID(%q) found = true, want false", tt.authID)
			}
			if got != nil {
				t.Errorf("GetByID(%q) auth = %+v, want nil", tt.authID, got)
			}
		})
	}
}

func TestManager_List_ReturnsClones(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		numAuths  int
		wantCount int
	}{
		{
			name:      "empty manager returns empty list",
			numAuths:  0,
			wantCount: 0,
		},
		{
			name:      "single auth returns list of one",
			numAuths:  1,
			wantCount: 1,
		},
		{
			name:      "multiple auths returns all",
			numAuths:  5,
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)

			// Register auths
			for i := 0; i < tt.numAuths; i++ {
				auth := &Auth{
					ID:         "auth-" + string(rune('a'+i)),
					Provider:   "testprovider",
					Attributes: map[string]string{"index": string(rune('0' + i))},
				}
				m.mu.Lock()
				m.auths[auth.ID] = auth
				m.mu.Unlock()
			}

			list := m.List()

			if len(list) != tt.wantCount {
				t.Errorf("List() returned %d auths, want %d", len(list), tt.wantCount)
			}

			if tt.numAuths > 0 {
				// Verify returned items are clones
				for _, got := range list {
					m.mu.RLock()
					original := m.auths[got.ID]
					m.mu.RUnlock()

					if got == original {
						t.Errorf("List() returned original pointer for auth %s, expected clone", got.ID)
					}

					// Modify returned auth and verify original unchanged
					got.Attributes["modified"] = "true"
					if original.Attributes["modified"] == "true" {
						t.Errorf("modifying list item affected original auth %s", got.ID)
					}
				}
			}
		})
	}
}

// --- Additional edge case tests ---

func TestManager_RegisterExecutor_NilExecutor(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, nil, nil)
	m.RegisterExecutor(nil) // Should not panic

	if len(m.executors) != 0 {
		t.Error("RegisterExecutor(nil) should not add to executors map")
	}
}

func TestManager_UnregisterExecutor(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, nil, nil)
	executor := &mockExecutor{provider: "testprovider"}
	m.RegisterExecutor(executor)

	if len(m.executors) != 1 {
		t.Fatal("executor not registered")
	}

	m.UnregisterExecutor("testprovider")

	if len(m.executors) != 0 {
		t.Error("UnregisterExecutor did not remove executor")
	}

	// Unregistering non-existent provider should not panic
	m.UnregisterExecutor("nonexistent")
	m.UnregisterExecutor("")
	m.UnregisterExecutor("   ")
}

func TestManager_SetRetryConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		retry            int
		maxRetryInterval time.Duration
		wantRetry        int
		wantInterval     time.Duration
	}{
		{
			name:             "positive values are stored",
			retry:            3,
			maxRetryInterval: 5 * time.Second,
			wantRetry:        3,
			wantInterval:     5 * time.Second,
		},
		{
			name:             "negative retry clamped to zero",
			retry:            -1,
			maxRetryInterval: time.Second,
			wantRetry:        0,
			wantInterval:     time.Second,
		},
		{
			name:             "negative interval clamped to zero",
			retry:            1,
			maxRetryInterval: -time.Second,
			wantRetry:        1,
			wantInterval:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(nil, nil, nil)
			m.SetRetryConfig(tt.retry, tt.maxRetryInterval, 0)

			gotRetry, _, gotInterval := m.retrySettings()

			if gotRetry != tt.wantRetry {
				t.Errorf("retrySettings() retry = %d, want %d", gotRetry, tt.wantRetry)
			}
			if gotInterval != tt.wantInterval {
				t.Errorf("retrySettings() interval = %v, want %v", gotInterval, tt.wantInterval)
			}
		})
	}
}

func TestManager_SetSelector(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, &mockSelector{}, nil)

	// Setting nil uses default
	m.SetSelector(nil)
	if _, ok := m.selector.(*RoundRobinSelector); !ok {
		t.Error("SetSelector(nil) should use RoundRobinSelector")
	}

	// Setting custom selector
	custom := &FillFirstSelector{}
	m.SetSelector(custom)
	if m.selector != custom {
		t.Error("SetSelector did not set custom selector")
	}
}

func TestManager_Load(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		storeAuths []*Auth
		storeErr   error
		wantErr    bool
		wantCount  int
	}{
		{
			name: "loads auths from store",
			storeAuths: []*Auth{
				{ID: "auth-1", Provider: "test"},
				{ID: "auth-2", Provider: "test"},
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:       "empty store returns empty auths",
			storeAuths: nil,
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:     "store error propagates",
			storeErr: errors.New("store error"),
			wantErr:  true,
		},
		{
			name: "nil auths in store are skipped",
			storeAuths: []*Auth{
				{ID: "auth-1", Provider: "test"},
				nil,
				{ID: "", Provider: "test"}, // empty ID should be skipped
			},
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore()
			store.listErr = tt.storeErr
			for _, auth := range tt.storeAuths {
				if auth != nil && auth.ID != "" {
					store.auths[auth.ID] = auth
				}
			}

			m := NewManager(store, nil, nil)
			// Pre-populate to verify Load replaces
			m.mu.Lock()
			m.auths["pre-existing"] = &Auth{ID: "pre-existing"}
			m.mu.Unlock()

			err := m.Load(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("Load() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}

			m.mu.RLock()
			count := len(m.auths)
			m.mu.RUnlock()

			if count != tt.wantCount {
				t.Errorf("Load() resulted in %d auths, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestManager_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing *Auth
		update   *Auth
		wantNil  bool
	}{
		{
			name:    "nil auth returns nil",
			update:  nil,
			wantNil: true,
		},
		{
			name:    "empty ID returns nil",
			update:  &Auth{ID: "", Provider: "test"},
			wantNil: true,
		},
		{
			name:     "updates existing auth",
			existing: &Auth{ID: "test-id", Provider: "test", Status: StatusActive},
			update:   &Auth{ID: "test-id", Provider: "test", Status: StatusError},
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := &mockHook{}
			m := NewManager(nil, nil, hook)
			ctx := context.Background()

			if tt.existing != nil {
				m.mu.Lock()
				m.auths[tt.existing.ID] = tt.existing
				m.mu.Unlock()
			}

			result, err := m.Update(ctx, tt.update)

			if err != nil {
				t.Fatalf("Update() unexpected error: %v", err)
			}

			if tt.wantNil {
				if result != nil {
					t.Errorf("Update() expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Update() returned nil unexpectedly")
			}

			// Verify hook was called
			hook.mu.Lock()
			updatedCount := len(hook.updatedAuths)
			hook.mu.Unlock()

			if updatedCount == 0 {
				t.Error("Update() did not trigger OnAuthUpdated hook")
			}
		})
	}
}

// Helper functions

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
