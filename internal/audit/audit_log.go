package audit

import (
	"context"
	"sync"
	"time"
)

type AuditEntry struct {
	Action    string
	EntityID  string
	Timestamp time.Time
}

type InMemoryAuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func NewInMemoryAuditLog() *InMemoryAuditLog {
	return &InMemoryAuditLog{}
}

func (l *InMemoryAuditLog) Record(ctx context.Context, action, entityID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, AuditEntry{Action: action, EntityID: entityID, Timestamp: time.Now()})
}

func (l *InMemoryAuditLog) List() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]AuditEntry, len(l.entries))
	copy(result, l.entries)
	return result
}
