package application_test

import (
	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestBug03ConcurrentLockSingleCommit(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	repo, err := repository.NewFileRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	j, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := recovery.NewRecoveryManager(store, j)
	if err != nil {
		t.Fatal(err)
	}
	log := audit.NewInMemoryAuditLog()
	svc := application.NewCertificateService(repo, rec, j, log)
	ctx := context.Background()
	id, err := svc.SubmitApplication(ctx, "example.test", "RSA-2048", "issuer")
	if err != nil {
		t.Fatal(err)
	}

	const workers = 12
	start := make(chan struct{})
	results := make(chan error, workers)
	var ready sync.WaitGroup
	ready.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			ready.Done()
			<-start
			results <- svc.LockApplication(ctx, id)
		}()
	}
	ready.Wait()
	close(start)

	successes := 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, domain.ErrInvalidTransition) &&
			!errors.Is(err, domain.ErrOptimisticConflict) {
			t.Fatalf("unexpected concurrent lock error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("lock committed %d times, want exactly once", successes)
	}

	got, err := repo.Applications.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ApplicationLocked || got.Version != 2 {
		t.Fatalf("persisted application diverged: %#v", got)
	}
	lockAudits := 0
	for _, entry := range log.List() {
		if entry.Action == "LockApplication" {
			lockAudits++
		}
	}
	if lockAudits != 1 {
		t.Fatalf("recorded %d lock audit entries, want one committed transition", lockAudits)
	}
}
