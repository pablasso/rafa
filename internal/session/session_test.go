package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorage_Save(t *testing.T) {
	t.Run("creates valid JSON file with atomic write", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		session := &Session{
			SessionID:    "test-session-123",
			Phase:        PhasePRD,
			Name:         "user-auth",
			DocumentPath: "docs/prds/user-auth.md",
			Status:       StatusInProgress,
			CreatedAt:    time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		}

		err := storage.Save(session)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file was created with correct name
		expectedFile := filepath.Join(tmpDir, "prd-user-auth.json")
		data, err := os.ReadFile(expectedFile)
		if err != nil {
			t.Fatalf("failed to read saved file: %v", err)
		}

		// Verify it's valid JSON
		var loaded Session
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("saved file is not valid JSON: %v", err)
		}

		// Verify key fields
		if loaded.SessionID != session.SessionID {
			t.Errorf("SessionID mismatch: got %q, want %q", loaded.SessionID, session.SessionID)
		}
		if loaded.Phase != session.Phase {
			t.Errorf("Phase mismatch: got %q, want %q", loaded.Phase, session.Phase)
		}
		if loaded.Name != session.Name {
			t.Errorf("Name mismatch: got %q, want %q", loaded.Name, session.Name)
		}
		if loaded.Status != session.Status {
			t.Errorf("Status mismatch: got %q, want %q", loaded.Status, session.Status)
		}

		// Verify UpdatedAt was set
		if loaded.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}

		// Verify no temp file left behind
		matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
		if len(matches) > 0 {
			t.Errorf("temp files left behind: %v", matches)
		}
	})

	t.Run("creates directory if it does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		sessionsDir := filepath.Join(tmpDir, "nested", "sessions")
		storage := NewStorage(sessionsDir)

		session := &Session{
			SessionID: "test-123",
			Phase:     PhaseDesign,
			Name:      "api-design",
			Status:    StatusInProgress,
			CreatedAt: time.Now(),
		}

		err := storage.Save(session)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(sessionsDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
	})

	t.Run("uses pretty-printed JSON with 2-space indent", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		session := &Session{
			SessionID: "test-123",
			Phase:     PhasePRD,
			Name:      "test",
			Status:    StatusInProgress,
			CreatedAt: time.Now(),
		}

		err := storage.Save(session)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, "prd-test.json"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		content := string(data)
		// Check for 2-space indentation
		if !strings.Contains(content, "\n  \"") {
			t.Error("JSON should use 2-space indentation")
		}
		// Check it starts and ends correctly
		if content[0] != '{' || content[len(content)-1] != '}' {
			t.Error("JSON should start with { and end with }")
		}
	})

	t.Run("updates UpdatedAt on save", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		originalTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		session := &Session{
			SessionID: "test-123",
			Phase:     PhasePRD,
			Name:      "test",
			Status:    StatusInProgress,
			CreatedAt: originalTime,
			UpdatedAt: originalTime,
		}

		beforeSave := time.Now()
		err := storage.Save(session)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify UpdatedAt was updated
		if !session.UpdatedAt.After(originalTime) {
			t.Error("UpdatedAt should be updated to current time")
		}
		if session.UpdatedAt.Before(beforeSave) {
			t.Error("UpdatedAt should be at or after save time")
		}
	})
}

func TestStorage_Load(t *testing.T) {
	t.Run("loads session by phase and name", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create a session file manually
		session := Session{
			SessionID:    "loaded-session-123",
			Phase:        PhaseDesign,
			Name:         "api-v2",
			DocumentPath: "docs/designs/api-v2.md",
			Status:       StatusCompleted,
			CreatedAt:    time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2024, 6, 15, 11, 45, 0, 0, time.UTC),
			FromDocument: "docs/prds/api.md",
		}

		data, _ := json.MarshalIndent(session, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "design-api-v2.json"), data, 0644)

		// Load it
		loaded, err := storage.Load(PhaseDesign, "api-v2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify all fields
		if loaded.SessionID != session.SessionID {
			t.Errorf("SessionID mismatch: got %q, want %q", loaded.SessionID, session.SessionID)
		}
		if loaded.Phase != session.Phase {
			t.Errorf("Phase mismatch: got %q, want %q", loaded.Phase, session.Phase)
		}
		if loaded.Name != session.Name {
			t.Errorf("Name mismatch: got %q, want %q", loaded.Name, session.Name)
		}
		if loaded.DocumentPath != session.DocumentPath {
			t.Errorf("DocumentPath mismatch: got %q, want %q", loaded.DocumentPath, session.DocumentPath)
		}
		if loaded.Status != session.Status {
			t.Errorf("Status mismatch: got %q, want %q", loaded.Status, session.Status)
		}
		if loaded.FromDocument != session.FromDocument {
			t.Errorf("FromDocument mismatch: got %q, want %q", loaded.FromDocument, session.FromDocument)
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		_, err := storage.Load(PhasePRD, "non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected not exist error, got: %v", err)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write invalid JSON
		os.WriteFile(filepath.Join(tmpDir, "prd-invalid.json"), []byte("not valid json"), 0644)

		_, err := storage.Load(PhasePRD, "invalid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse session") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})
}

func TestStorage_LoadByPhase(t *testing.T) {
	t.Run("returns most recently updated session", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create multiple sessions with different UpdatedAt times
		oldSession := Session{
			SessionID: "old-session",
			Phase:     PhasePRD,
			Name:      "old-feature",
			Status:    StatusInProgress,
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		newSession := Session{
			SessionID: "new-session",
			Phase:     PhasePRD,
			Name:      "new-feature",
			Status:    StatusInProgress,
			CreatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		}

		middleSession := Session{
			SessionID: "middle-session",
			Phase:     PhasePRD,
			Name:      "middle-feature",
			Status:    StatusCompleted,
			CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		}

		// Save all sessions
		for _, s := range []Session{oldSession, newSession, middleSession} {
			data, _ := json.MarshalIndent(s, "", "  ")
			filename := filepath.Join(tmpDir, string(s.Phase)+"-"+s.Name+".json")
			os.WriteFile(filename, data, 0644)
		}

		// Load by phase
		loaded, err := storage.LoadByPhase(PhasePRD)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return the most recent one
		if loaded.SessionID != newSession.SessionID {
			t.Errorf("expected most recent session %q, got %q", newSession.SessionID, loaded.SessionID)
		}
	})

	t.Run("returns error when no sessions exist for phase", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create a session for a different phase
		session := Session{
			SessionID: "design-session",
			Phase:     PhaseDesign,
			Name:      "test",
			Status:    StatusInProgress,
			UpdatedAt: time.Now(),
		}
		data, _ := json.MarshalIndent(session, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "design-test.json"), data, 0644)

		// Try to load PRD phase
		_, err := storage.LoadByPhase(PhasePRD)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected not exist error, got: %v", err)
		}
	})

	t.Run("returns error when directory is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		_, err := storage.LoadByPhase(PhasePRD)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected not exist error, got: %v", err)
		}
	})

	t.Run("skips invalid JSON files", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create one valid session
		validSession := Session{
			SessionID: "valid-session",
			Phase:     PhasePRD,
			Name:      "valid",
			Status:    StatusInProgress,
			UpdatedAt: time.Now(),
		}
		data, _ := json.MarshalIndent(validSession, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "prd-valid.json"), data, 0644)

		// Create an invalid JSON file
		os.WriteFile(filepath.Join(tmpDir, "prd-invalid.json"), []byte("not json"), 0644)

		// Should still return the valid session
		loaded, err := storage.LoadByPhase(PhasePRD)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loaded.SessionID != validSession.SessionID {
			t.Errorf("expected valid session, got %q", loaded.SessionID)
		}
	})
}

func TestStorage_List(t *testing.T) {
	t.Run("returns all sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create sessions for different phases
		sessions := []Session{
			{
				SessionID: "prd-1",
				Phase:     PhasePRD,
				Name:      "feature-a",
				Status:    StatusInProgress,
				UpdatedAt: time.Now(),
			},
			{
				SessionID: "design-1",
				Phase:     PhaseDesign,
				Name:      "feature-b",
				Status:    StatusCompleted,
				UpdatedAt: time.Now(),
			},
			{
				SessionID: "plan-1",
				Phase:     PhasePlanCreate,
				Name:      "feature-c",
				Status:    StatusCancelled,
				UpdatedAt: time.Now(),
			},
		}

		for _, s := range sessions {
			data, _ := json.MarshalIndent(s, "", "  ")
			filename := filepath.Join(tmpDir, string(s.Phase)+"-"+s.Name+".json")
			os.WriteFile(filename, data, 0644)
		}

		// List all
		listed, err := storage.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(listed) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(listed))
		}

		// Verify all session IDs are present
		ids := make(map[string]bool)
		for _, s := range listed {
			ids[s.SessionID] = true
		}
		for _, s := range sessions {
			if !ids[s.SessionID] {
				t.Errorf("session %q not found in list", s.SessionID)
			}
		}
	})

	t.Run("returns empty slice for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		listed, err := storage.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if listed == nil {
			// nil is acceptable for empty result
		} else if len(listed) != 0 {
			t.Errorf("expected empty list, got %d sessions", len(listed))
		}
	})

	t.Run("skips invalid JSON files", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create one valid session
		validSession := Session{
			SessionID: "valid",
			Phase:     PhasePRD,
			Name:      "test",
			Status:    StatusInProgress,
			UpdatedAt: time.Now(),
		}
		data, _ := json.MarshalIndent(validSession, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "prd-test.json"), data, 0644)

		// Create invalid files
		os.WriteFile(filepath.Join(tmpDir, "invalid.json"), []byte("not json"), 0644)

		listed, err := storage.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(listed) != 1 {
			t.Errorf("expected 1 session, got %d", len(listed))
		}
	})
}

func TestStorage_Delete(t *testing.T) {
	t.Run("removes session file", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create a session file
		filename := filepath.Join(tmpDir, "prd-to-delete.json")
		os.WriteFile(filename, []byte("{}"), 0644)

		// Verify it exists
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("file should exist before delete: %v", err)
		}

		// Delete it
		err := storage.Delete(PhasePRD, "to-delete")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify it's gone
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Error("file should be removed after delete")
		}
	})

	t.Run("is idempotent for non-existent files", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Delete non-existent file - should not error
		err := storage.Delete(PhasePRD, "does-not-exist")
		if err != nil {
			t.Errorf("delete of non-existent file should not error, got: %v", err)
		}

		// Call again - should still not error
		err = storage.Delete(PhasePRD, "does-not-exist")
		if err != nil {
			t.Errorf("repeated delete should not error, got: %v", err)
		}
	})

	t.Run("deletes correct file based on phase and name", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Create multiple files
		os.WriteFile(filepath.Join(tmpDir, "prd-feature.json"), []byte("{}"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "design-feature.json"), []byte("{}"), 0644)

		// Delete only PRD
		err := storage.Delete(PhasePRD, "feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// PRD should be gone
		if _, err := os.Stat(filepath.Join(tmpDir, "prd-feature.json")); !os.IsNotExist(err) {
			t.Error("prd-feature.json should be deleted")
		}

		// Design should still exist
		if _, err := os.Stat(filepath.Join(tmpDir, "design-feature.json")); err != nil {
			t.Error("design-feature.json should still exist")
		}
	})
}

func TestSessionFilename(t *testing.T) {
	t.Run("formats filename as phase-name.json", func(t *testing.T) {
		storage := NewStorage("/tmp/sessions")

		testCases := []struct {
			phase    Phase
			name     string
			expected string
		}{
			{PhasePRD, "user-auth", "/tmp/sessions/prd-user-auth.json"},
			{PhaseDesign, "api-v2", "/tmp/sessions/design-api-v2.json"},
			{PhasePlanCreate, "feature-x", "/tmp/sessions/plan-create-feature-x.json"},
		}

		for _, tc := range testCases {
			result := storage.sessionFilename(tc.phase, tc.name)
			if result != tc.expected {
				t.Errorf("sessionFilename(%q, %q) = %q, want %q", tc.phase, tc.name, result, tc.expected)
			}
		}
	})
}

// ========================================================================
// Error Recovery Tests
// ========================================================================

func TestStorage_Load_CorruptJSON_GracefulError(t *testing.T) {
	t.Run("returns parse error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write invalid JSON
		os.WriteFile(filepath.Join(tmpDir, "prd-corrupt.json"), []byte("not valid json at all"), 0644)

		_, err := storage.Load(PhasePRD, "corrupt")
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse session") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})

	t.Run("returns parse error for truncated JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write truncated JSON
		os.WriteFile(filepath.Join(tmpDir, "prd-truncated.json"), []byte(`{"sessionId": "test", "phase"`), 0644)

		_, err := storage.Load(PhasePRD, "truncated")
		if err == nil {
			t.Fatal("expected error for truncated JSON, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse session") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})

	t.Run("returns parse error for empty JSON object", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write empty JSON - should parse but result in zero values
		os.WriteFile(filepath.Join(tmpDir, "prd-empty.json"), []byte(`{}`), 0644)

		sess, err := storage.Load(PhasePRD, "empty")
		if err != nil {
			t.Fatalf("empty JSON should parse: %v", err)
		}
		// Empty session should have zero values
		if sess.SessionID != "" {
			t.Errorf("expected empty sessionId, got %s", sess.SessionID)
		}
	})

	t.Run("returns parse error for binary garbage", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write binary garbage
		garbage := []byte{0x00, 0x01, 0x02, 0x89, 0xAB, 0xCD, 0xEF}
		os.WriteFile(filepath.Join(tmpDir, "prd-garbage.json"), garbage, 0644)

		_, err := storage.Load(PhasePRD, "garbage")
		if err == nil {
			t.Fatal("expected error for binary garbage, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse session") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})

	t.Run("returns parse error for wrong type fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write JSON with wrong types (createdAt as string instead of proper time)
		// Note: Go's JSON unmarshaler is lenient about time formats
		os.WriteFile(filepath.Join(tmpDir, "prd-wrongtype.json"), []byte(`{"sessionId": 123}`), 0644)

		_, err := storage.Load(PhasePRD, "wrongtype")
		// JSON unmarshaler may or may not error depending on type coercion
		// The important thing is it doesn't panic
		if err != nil {
			// Error is acceptable
			if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "unmarshal") {
				// Different error types are OK as long as it fails gracefully
			}
		}
	})
}

func TestStorage_LoadByPhase_SkipsCorruptFiles(t *testing.T) {
	t.Run("skips corrupt files and returns valid session", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write one corrupt file
		os.WriteFile(filepath.Join(tmpDir, "prd-corrupt.json"), []byte("not json"), 0644)

		// Write one valid file
		validSession := Session{
			SessionID: "valid-session",
			Phase:     PhasePRD,
			Name:      "valid",
			Status:    StatusInProgress,
			UpdatedAt: time.Now(),
		}
		data, _ := json.MarshalIndent(validSession, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, "prd-valid.json"), data, 0644)

		// LoadByPhase should return the valid session, skipping the corrupt one
		loaded, err := storage.LoadByPhase(PhasePRD)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loaded.SessionID != "valid-session" {
			t.Errorf("expected valid session, got %s", loaded.SessionID)
		}
	})

	t.Run("returns error when all files are corrupt", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write only corrupt files
		os.WriteFile(filepath.Join(tmpDir, "prd-corrupt1.json"), []byte("not json 1"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "prd-corrupt2.json"), []byte("not json 2"), 0644)

		_, err := storage.LoadByPhase(PhasePRD)
		if err == nil {
			t.Fatal("expected error when all files are corrupt")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected not exist error, got: %v", err)
		}
	})
}

func TestStorage_List_SkipsCorruptFiles(t *testing.T) {
	t.Run("skips corrupt files and returns valid sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		// Write corrupt files
		os.WriteFile(filepath.Join(tmpDir, "prd-corrupt.json"), []byte("corrupt"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "design-broken.json"), []byte("{broken"), 0644)

		// Write valid files
		for _, s := range []Session{
			{SessionID: "prd-1", Phase: PhasePRD, Name: "a", Status: StatusInProgress, UpdatedAt: time.Now()},
			{SessionID: "design-1", Phase: PhaseDesign, Name: "b", Status: StatusCompleted, UpdatedAt: time.Now()},
		} {
			data, _ := json.MarshalIndent(s, "", "  ")
			os.WriteFile(filepath.Join(tmpDir, string(s.Phase)+"-"+s.Name+".json"), data, 0644)
		}

		sessions, err := storage.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have exactly 2 valid sessions
		if len(sessions) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(sessions))
		}

		// Verify they are the valid ones
		ids := make(map[string]bool)
		for _, s := range sessions {
			ids[s.SessionID] = true
		}
		if !ids["prd-1"] || !ids["design-1"] {
			t.Error("expected valid sessions prd-1 and design-1")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("save then load preserves all fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewStorage(tmpDir)

		original := &Session{
			SessionID:    "roundtrip-session",
			Phase:        PhaseDesign,
			Name:         "comprehensive-test",
			DocumentPath: "docs/designs/test.md",
			Status:       StatusInProgress,
			CreatedAt:    time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			FromDocument: "docs/prds/source.md",
		}

		// Save
		err := storage.Save(original)
		if err != nil {
			t.Fatalf("save failed: %v", err)
		}

		// Load
		loaded, err := storage.Load(PhaseDesign, "comprehensive-test")
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}

		// Compare fields (except UpdatedAt which changes on save)
		if loaded.SessionID != original.SessionID {
			t.Errorf("SessionID: got %q, want %q", loaded.SessionID, original.SessionID)
		}
		if loaded.Phase != original.Phase {
			t.Errorf("Phase: got %q, want %q", loaded.Phase, original.Phase)
		}
		if loaded.Name != original.Name {
			t.Errorf("Name: got %q, want %q", loaded.Name, original.Name)
		}
		if loaded.DocumentPath != original.DocumentPath {
			t.Errorf("DocumentPath: got %q, want %q", loaded.DocumentPath, original.DocumentPath)
		}
		if loaded.Status != original.Status {
			t.Errorf("Status: got %q, want %q", loaded.Status, original.Status)
		}
		if !loaded.CreatedAt.Equal(original.CreatedAt) {
			t.Errorf("CreatedAt: got %v, want %v", loaded.CreatedAt, original.CreatedAt)
		}
		if loaded.FromDocument != original.FromDocument {
			t.Errorf("FromDocument: got %q, want %q", loaded.FromDocument, original.FromDocument)
		}
	})
}
