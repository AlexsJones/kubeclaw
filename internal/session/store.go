// Package session provides session persistence and transcript storage
// backed by PostgreSQL with pgvector for memory embeddings.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Session represents an active conversation session.
type Session struct {
	ID           string            `json:"id"`
	InstanceName string            `json:"instanceName"`
	ChannelType  string            `json:"channelType"`
	ChatID       string            `json:"chatId"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// TranscriptEvent represents a single event in the conversation transcript.
type TranscriptEvent struct {
	ID        string          `json:"id"`
	SessionID string          `json:"sessionId"`
	Role      string          `json:"role"` // user, assistant, system, tool
	Content   string          `json:"content"`
	ToolName  string          `json:"toolName,omitempty"`
	ToolInput json.RawMessage `json:"toolInput,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// Store provides session and transcript persistence using PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new session store connected to PostgreSQL.
func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close shuts down the database connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// CreateSession creates a new session.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	now := time.Now()
	sess.CreatedAt = now
	sess.UpdatedAt = now

	metadataJSON, err := json.Marshal(sess.Metadata)
	if err != nil {
		return fmt.Errorf("marshalling metadata: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (id, instance_name, channel_type, chat_id, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sess.ID, sess.InstanceName, sess.ChannelType, sess.ChatID, metadataJSON, sess.CreatedAt, sess.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx,
		`SELECT id, instance_name, channel_type, chat_id, metadata, created_at, updated_at
		 FROM sessions WHERE id = $1`, id,
	).Scan(&sess.ID, &sess.InstanceName, &sess.ChannelType, &sess.ChatID, &metadataJSON, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &sess.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshalling metadata: %w", err)
		}
	}

	return sess, nil
}

// FindSessionByChatID looks up a session by channel type and chat ID.
func (s *Store) FindSessionByChatID(ctx context.Context, channelType, chatID string) (*Session, error) {
	sess := &Session{}
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx,
		`SELECT id, instance_name, channel_type, chat_id, metadata, created_at, updated_at
		 FROM sessions WHERE channel_type = $1 AND chat_id = $2
		 ORDER BY updated_at DESC LIMIT 1`, channelType, chatID,
	).Scan(&sess.ID, &sess.InstanceName, &sess.ChannelType, &sess.ChatID, &metadataJSON, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying session by chat: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &sess.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshalling metadata: %w", err)
		}
	}

	return sess, nil
}

// AppendTranscript appends an event to the session transcript.
func (s *Store) AppendTranscript(ctx context.Context, event *TranscriptEvent) error {
	event.Timestamp = time.Now()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO transcript_events (id, session_id, role, content, tool_name, tool_input, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		event.ID, event.SessionID, event.Role, event.Content,
		event.ToolName, event.ToolInput, event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("inserting transcript event: %w", err)
	}

	// Touch the session's updated_at
	_, err = s.pool.Exec(ctx,
		`UPDATE sessions SET updated_at = $1 WHERE id = $2`, time.Now(), event.SessionID)
	if err != nil {
		return fmt.Errorf("updating session timestamp: %w", err)
	}

	return nil
}

// GetTranscript retrieves all transcript events for a session, ordered by time.
func (s *Store) GetTranscript(ctx context.Context, sessionID string) ([]TranscriptEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, role, content, tool_name, tool_input, created_at
		 FROM transcript_events WHERE session_id = $1 ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying transcript: %w", err)
	}
	defer rows.Close()

	var events []TranscriptEvent
	for rows.Next() {
		var e TranscriptEvent
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Role, &e.Content, &e.ToolName, &e.ToolInput, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scanning transcript event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// Manager provides higher-level session management operations.
type Manager struct {
	store *Store
}

// NewManager creates a new session Manager.
func NewManager(store *Store) *Manager {
	return &Manager{store: store}
}

// GetOrCreateSession finds an existing session or creates a new one.
func (m *Manager) GetOrCreateSession(ctx context.Context, instanceName, channelType, chatID string) (*Session, error) {
	sess, err := m.store.FindSessionByChatID(ctx, channelType, chatID)
	if err == nil {
		return sess, nil
	}

	// Create new session
	newSess := &Session{
		ID:           fmt.Sprintf("sess-%s-%s-%d", channelType, chatID, time.Now().UnixNano()),
		InstanceName: instanceName,
		ChannelType:  channelType,
		ChatID:       chatID,
	}

	if err := m.store.CreateSession(ctx, newSess); err != nil {
		return nil, err
	}

	return newSess, nil
}
