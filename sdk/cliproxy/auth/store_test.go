package auth_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestFileStore_Save_CreatesFile(t *testing.T) {
	tests := []struct {
		name     string
		auth     *cliproxyauth.Auth
		wantErr  bool
		validate func(t *testing.T, tmpDir string, authRecord *cliproxyauth.Auth)
	}{
		{
			name: "creates new file with metadata",
			auth: &cliproxyauth.Auth{
				ID:       "test-auth.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"email": "test@example.com",
					"type":  "test-provider",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
			validate: func(t *testing.T, tmpDir string, authRecord *cliproxyauth.Auth) {
				path := filepath.Join(tmpDir, "test-auth.json")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("expected file %s to exist", path)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}
				var metadata map[string]any
				if err := json.Unmarshal(data, &metadata); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if metadata["email"] != "test@example.com" {
					t.Errorf("expected email 'test@example.com', got %v", metadata["email"])
				}
			},
		},
		{
			name: "creates file in subdirectory",
			auth: &cliproxyauth.Auth{
				ID:       "subdir/nested-auth.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"type": "test-provider",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, tmpDir string, authRecord *cliproxyauth.Auth) {
				path := filepath.Join(tmpDir, "subdir", "nested-auth.json")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("expected file %s to exist", path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			_, err := store.Save(ctx, tt.auth)

			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, tmpDir, tt.auth)
			}
		})
	}
}

func TestFileStore_Save_UpdatesExisting(t *testing.T) {
	tests := []struct {
		name         string
		initialAuth  *cliproxyauth.Auth
		updatedAuth  *cliproxyauth.Auth
		wantErr      bool
		validateFunc func(t *testing.T, tmpDir string)
	}{
		{
			name: "updates existing file with new metadata",
			initialAuth: &cliproxyauth.Auth{
				ID:       "update-test.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"email": "initial@example.com",
					"type":  "test-provider",
				},
			},
			updatedAuth: &cliproxyauth.Auth{
				ID:       "update-test.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"email": "updated@example.com",
					"type":  "test-provider",
				},
			},
			wantErr: false,
			validateFunc: func(t *testing.T, tmpDir string) {
				path := filepath.Join(tmpDir, "update-test.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}
				var metadata map[string]any
				if err := json.Unmarshal(data, &metadata); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if metadata["email"] != "updated@example.com" {
					t.Errorf("expected email 'updated@example.com', got %v", metadata["email"])
				}
			},
		},
		{
			name: "updates file preserving unmodified fields",
			initialAuth: &cliproxyauth.Auth{
				ID:       "preserve-test.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"email":      "test@example.com",
					"type":       "test-provider",
					"extra_data": "should-be-replaced",
				},
			},
			updatedAuth: &cliproxyauth.Auth{
				ID:       "preserve-test.json",
				Provider: "test-provider",
				Metadata: map[string]any{
					"email":      "test@example.com",
					"type":       "test-provider",
					"extra_data": "new-value",
				},
			},
			wantErr: false,
			validateFunc: func(t *testing.T, tmpDir string) {
				path := filepath.Join(tmpDir, "preserve-test.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}
				var metadata map[string]any
				if err := json.Unmarshal(data, &metadata); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if metadata["extra_data"] != "new-value" {
					t.Errorf("expected extra_data 'new-value', got %v", metadata["extra_data"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()

			// Save initial auth
			if _, err := store.Save(ctx, tt.initialAuth); err != nil {
				t.Fatalf("failed to save initial auth: %v", err)
			}

			// Update auth
			_, err := store.Save(ctx, tt.updatedAuth)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, tmpDir)
			}
		})
	}
}

func TestFileStore_Load_ExistingFile(t *testing.T) {
	tests := []struct {
		name         string
		setupFile    func(t *testing.T, tmpDir string)
		wantCount    int
		wantProvider string
		wantEmail    string
	}{
		{
			name: "loads single auth file",
			setupFile: func(t *testing.T, tmpDir string) {
				data := map[string]any{
					"type":  "test-provider",
					"email": "loaded@example.com",
				}
				raw, _ := json.Marshal(data)
				path := filepath.Join(tmpDir, "load-test.json")
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Fatalf("failed to write setup file: %v", err)
				}
			},
			wantCount:    1,
			wantProvider: "test-provider",
			wantEmail:    "loaded@example.com",
		},
		{
			name: "loads multiple auth files",
			setupFile: func(t *testing.T, tmpDir string) {
				for i, email := range []string{"user1@example.com", "user2@example.com"} {
					data := map[string]any{
						"type":  "test-provider",
						"email": email,
					}
					raw, _ := json.Marshal(data)
					fileName := filepath.Join(tmpDir, "auth"+string(rune('1'+i))+".json")
					if err := os.WriteFile(fileName, raw, 0o600); err != nil {
						t.Fatalf("failed to write setup file: %v", err)
					}
				}
			},
			wantCount:    2,
			wantProvider: "test-provider",
			wantEmail:    "", // Multiple emails, don't check specific
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFile(t, tmpDir)

			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			auths, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if len(auths) != tt.wantCount {
				t.Errorf("List() returned %d auths, want %d", len(auths), tt.wantCount)
			}

			if tt.wantCount == 1 && len(auths) == 1 {
				if auths[0].Provider != tt.wantProvider {
					t.Errorf("Provider = %v, want %v", auths[0].Provider, tt.wantProvider)
				}
				if tt.wantEmail != "" {
					email, _ := auths[0].Metadata["email"].(string)
					if email != tt.wantEmail {
						t.Errorf("Email = %v, want %v", email, tt.wantEmail)
					}
				}
			}
		})
	}
}

func TestFileStore_Load_NotFound(t *testing.T) {
	tests := []struct {
		name      string
		setupDir  func(t *testing.T, tmpDir string)
		wantCount int
		wantErr   bool
	}{
		{
			name: "returns empty list for non-existent directory",
			setupDir: func(t *testing.T, tmpDir string) {
				// Directory does not contain any files, no setup needed
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "skips non-json files",
			setupDir: func(t *testing.T, tmpDir string) {
				if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not json"), 0o600); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("yaml: true"), 0o600); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "skips malformed json files",
			setupDir: func(t *testing.T, tmpDir string) {
				if err := os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte("{invalid json}"), 0o600); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupDir(t, tmpDir)

			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			auths, err := store.List(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(auths) != tt.wantCount {
				t.Errorf("List() returned %d auths, want %d", len(auths), tt.wantCount)
			}
		})
	}
}

func TestFileStore_List_AllFiles(t *testing.T) {
	tests := []struct {
		name      string
		setupDir  func(t *testing.T, tmpDir string)
		wantCount int
		wantIDs   []string
	}{
		{
			name: "lists all json files in directory",
			setupDir: func(t *testing.T, tmpDir string) {
				for _, name := range []string{"auth1.json", "auth2.json", "auth3.json"} {
					data := map[string]any{"type": "test-provider"}
					raw, _ := json.Marshal(data)
					if err := os.WriteFile(filepath.Join(tmpDir, name), raw, 0o600); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}
			},
			wantCount: 3,
			wantIDs:   []string{"auth1.json", "auth2.json", "auth3.json"},
		},
		{
			name: "lists files in nested directories",
			setupDir: func(t *testing.T, tmpDir string) {
				subDir := filepath.Join(tmpDir, "nested")
				if err := os.MkdirAll(subDir, 0o700); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				data := map[string]any{"type": "test-provider"}
				raw, _ := json.Marshal(data)
				if err := os.WriteFile(filepath.Join(tmpDir, "root.json"), raw, 0o600); err != nil {
					t.Fatalf("failed to write root file: %v", err)
				}
				if err := os.WriteFile(filepath.Join(subDir, "nested.json"), raw, 0o600); err != nil {
					t.Fatalf("failed to write nested file: %v", err)
				}
			},
			wantCount: 2,
			wantIDs:   []string{"root.json", "nested/nested.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupDir(t, tmpDir)

			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			auths, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if len(auths) != tt.wantCount {
				t.Errorf("List() returned %d auths, want %d", len(auths), tt.wantCount)
			}

			// Verify all expected IDs are present
			foundIDs := make(map[string]bool)
			for _, a := range auths {
				foundIDs[a.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				// Normalize path separator for cross-platform compatibility
				normalizedID := filepath.FromSlash(wantID)
				if !foundIDs[normalizedID] {
					t.Errorf("expected ID %s not found in results", wantID)
				}
			}
		})
	}
}

func TestFileStore_List_EmptyDir(t *testing.T) {
	tests := []struct {
		name     string
		setupDir func(t *testing.T, tmpDir string)
		wantLen  int
	}{
		{
			name: "returns empty slice for empty directory",
			setupDir: func(t *testing.T, tmpDir string) {
				// Empty directory, no setup needed
			},
			wantLen: 0,
		},
		{
			name: "returns empty slice when only empty subdirectories exist",
			setupDir: func(t *testing.T, tmpDir string) {
				if err := os.MkdirAll(filepath.Join(tmpDir, "empty1"), 0o700); err != nil {
					t.Fatalf("failed to create empty1: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(tmpDir, "empty2", "nested"), 0o700); err != nil {
					t.Fatalf("failed to create empty2/nested: %v", err)
				}
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupDir(t, tmpDir)

			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			auths, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if len(auths) != tt.wantLen {
				t.Errorf("List() returned %d auths, want %d", len(auths), tt.wantLen)
			}
		})
	}
}

func TestFileStore_Delete_RemovesFile(t *testing.T) {
	tests := []struct {
		name      string
		setupAuth *cliproxyauth.Auth
		deleteID  string
		wantErr   bool
		validate  func(t *testing.T, tmpDir string)
	}{
		{
			name: "deletes existing file by ID",
			setupAuth: &cliproxyauth.Auth{
				ID:       "delete-me.json",
				Provider: "test-provider",
				Metadata: map[string]any{"type": "test-provider"},
			},
			deleteID: "delete-me.json",
			wantErr:  false,
			validate: func(t *testing.T, tmpDir string) {
				path := filepath.Join(tmpDir, "delete-me.json")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("expected file %s to be deleted", path)
				}
			},
		},
		{
			name: "deletes file in subdirectory",
			setupAuth: &cliproxyauth.Auth{
				ID:       "subdir/nested-delete.json",
				Provider: "test-provider",
				Metadata: map[string]any{"type": "test-provider"},
			},
			deleteID: "subdir/nested-delete.json",
			wantErr:  false,
			validate: func(t *testing.T, tmpDir string) {
				path := filepath.Join(tmpDir, "subdir", "nested-delete.json")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("expected file %s to be deleted", path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()

			// Setup: create the file first
			if _, err := store.Save(ctx, tt.setupAuth); err != nil {
				t.Fatalf("failed to save setup auth: %v", err)
			}

			// Verify file exists before delete
			path := filepath.Join(tmpDir, filepath.FromSlash(tt.deleteID))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Fatalf("setup file %s does not exist", path)
			}

			// Delete
			err := store.Delete(ctx, tt.deleteID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, tmpDir)
			}
		})
	}
}

func TestFileStore_Delete_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		deleteID string
		wantErr  bool
	}{
		{
			name:     "no error when deleting non-existent file",
			deleteID: "does-not-exist.json",
			wantErr:  false,
		},
		{
			name:     "no error when deleting from non-existent subdirectory",
			deleteID: "nonexistent/path/file.json",
			wantErr:  false,
		},
		{
			name:     "error when id is empty",
			deleteID: "",
			wantErr:  true,
		},
		{
			name:     "error when id is whitespace only",
			deleteID: "   ",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := auth.NewFileTokenStore()
			store.SetBaseDir(tmpDir)

			ctx := context.Background()
			err := store.Delete(ctx, tt.deleteID)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
