package journal_test

import (
	"testing"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
)

func TestAppendAndRecover(t *testing.T) {
	dir := t.TempDir()
	j, _ := journal.NewFileJournal(dir)
	ev := domain.Event{Type: "test", EntityID: "1"}
	if err := j.Append(ev); err != nil {
		t.Fatal(err)
	}
	events, err := j.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}
