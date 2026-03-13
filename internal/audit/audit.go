// Package audit provides append-only structured audit logging.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

// EventType enumerates auditable event categories.
type EventType string

const (
	EventInit         EventType = "INIT"
	EventKeyGenerated EventType = "KEY_GENERATED"
	EventKeyRotated   EventType = "KEY_ROTATED"
	EventRegistered   EventType = "REGISTERED"
	EventRevoked      EventType = "REVOKED"
	EventTunnelStart  EventType = "TUNNEL_START"
	EventTunnelStop   EventType = "TUNNEL_STOP"
	EventAuthFailed   EventType = "AUTH_FAILED"
	EventHeartbeat    EventType = "HEARTBEAT"
	EventRateLimit    EventType = "RATE_LIMIT"
	EventPeerAllowed  EventType = "PEER_ALLOWED"
	EventPeerDenied   EventType = "PEER_DENIED"
)

// Entry is a single audit log record.
type Entry struct {
	Time    time.Time         `json:"time"`
	Event   EventType         `json:"event"`
	AgentID string            `json:"agent_id,omitempty"`
	Detail  string            `json:"detail,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Logger writes audit entries to a file.
type Logger struct {
	path    string
	agentID string
}

// New creates an audit Logger. If logPath is empty it uses the default location.
func New(agentID, logPath string) (*Logger, error) {
	if logPath == "" {
		var err error
		logPath, err = config.AuditLogPath()
		if err != nil {
			return nil, err
		}
	}
	return &Logger{path: logPath, agentID: agentID}, nil
}

// Log records an audit event.
func (l *Logger) Log(event EventType, detail string, meta map[string]string) error {
	entry := Entry{
		Time:    time.Now().UTC(),
		Event:   event,
		AgentID: l.agentID,
		Detail:  detail,
		Meta:    meta,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshalling audit entry: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening audit log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing audit log: %w", err)
	}
	return nil
}

// MustLog logs an audit event, printing a warning to stderr on failure.
func (l *Logger) MustLog(event EventType, detail string, meta map[string]string) {
	if err := l.Log(event, detail, meta); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: audit log failed: %v\n", err)
	}
}

// Read returns all audit log entries.
func Read() ([]Entry, error) {
	path, err := config.AuditLogPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading audit log: %w", err)
	}

	var entries []Entry
	dec := json.NewDecoder(
		// Wrap bytes in a reader
		jsonBytesReader(data),
	)
	for dec.More() {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func jsonBytesReader(data []byte) *byteReader {
	return &byteReader{data: data}
}
