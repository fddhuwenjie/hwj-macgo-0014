package recovery

import (
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBug07ValidJournalAndSnapshotRecover(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	if err = j.Append(domain.Event{Type: "issued", EntityID: "cert"}); err != nil {
		t.Fatal(err)
	}
	events, err := j.Recover()
	if err != nil || len(events) != 1 {
		t.Fatalf("journal recovery: %#v %v", events, err)
	}
	data := filepath.Join(dir, "data")
	for _, sub := range []string{"applications", "certificates", "targets", "deployments", "renewals", "revocations", "observations"} {
		if err = os.MkdirAll(filepath.Join(data, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(data, "applications", "a.json"), []byte(`{"ID":"a"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecoveryManager(data, j)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.TakeSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	snaps, err := r.listSnapshots()
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots: %#v %v", snaps, err)
	}
	if err = r.restoreSnapshot(snaps[0]); err != nil {
		t.Fatalf("restore valid snapshot: %v", err)
	}
}
