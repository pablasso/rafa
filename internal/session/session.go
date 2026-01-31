package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase represents the document creation phase.
type Phase string

const (
	PhasePRD        Phase = "prd"
	PhaseDesign     Phase = "design"
	PhasePlanCreate Phase = "plan-create"
)

// Status represents the session status.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCancelled  Status = "cancelled"
)

// Session tracks a document creation conversation.
type Session struct {
	SessionID    string    `json:"sessionId"`              // Claude's session ID for --resume
	Phase        Phase     `json:"phase"`                  // prd, design, or plan-create
	Name         string    `json:"name"`                   // User-friendly name (e.g., "user-auth")
	DocumentPath string    `json:"documentPath"`           // Path to output document
	Status       Status    `json:"status"`                 // in_progress, completed, or cancelled
	CreatedAt    time.Time `json:"createdAt"`              // When session was created
	UpdatedAt    time.Time `json:"updatedAt"`              // Last update time
	FromDocument string    `json:"fromDocument,omitempty"` // For designs created from PRD
}

// Storage manages session persistence.
type Storage struct {
	dir string
}

// NewStorage creates a storage instance for the given sessions directory.
func NewStorage(sessionsDir string) *Storage {
	return &Storage{dir: sessionsDir}
}

// Save persists a session to disk with atomic writes.
func (s *Storage) Save(session *Session) error {
	session.UpdatedAt = time.Now()

	// Ensure directory exists
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	filename := s.sessionFilename(session.Phase, session.Name)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpFile := filename + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session temp file: %w", err)
	}
	if err := os.Rename(tmpFile, filename); err != nil {
		// Clean up temp file on rename failure
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename session temp file: %w", err)
	}
	return nil
}

// Load retrieves a session by phase and name.
func (s *Storage) Load(phase Phase, name string) (*Session, error) {
	filename := s.sessionFilename(phase, name)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}
	return &session, nil
}

// LoadByPhase retrieves the most recently updated session for a phase.
func (s *Storage) LoadByPhase(phase Phase) (*Session, error) {
	pattern := filepath.Join(s.dir, string(phase)+"-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob sessions: %w", err)
	}

	if len(matches) == 0 {
		return nil, os.ErrNotExist
	}

	// Find most recently updated
	var latest *Session
	var latestTime time.Time

	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}

		if sess.UpdatedAt.After(latestTime) {
			latestTime = sess.UpdatedAt
			latest = &sess
		}
	}

	if latest == nil {
		return nil, os.ErrNotExist
	}
	return latest, nil
}

// List returns all sessions.
func (s *Storage) List() ([]*Session, error) {
	pattern := filepath.Join(s.dir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob sessions: %w", err)
	}

	var sessions []*Session
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		sessions = append(sessions, &sess)
	}

	return sessions, nil
}

// Delete removes a session file. Returns nil if the file doesn't exist (idempotent).
func (s *Storage) Delete(phase Phase, name string) error {
	filename := s.sessionFilename(phase, name)
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil // Idempotent: deleting non-existent file is not an error
	}
	return err
}

// sessionFilename returns the path for a session file.
// Format: <dir>/<phase>-<name>.json
func (s *Storage) sessionFilename(phase Phase, name string) string {
	return filepath.Join(s.dir, string(phase)+"-"+name+".json")
}
