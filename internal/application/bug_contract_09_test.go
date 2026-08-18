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
	"testing"
)

func TestBug09TransitionSentinelSurvivesLayers(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	repo, _ := repository.NewFileRepository(store)
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(store, j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	id, _ := svc.SubmitApplication(ctx, "example.test", "RSA-2048", "issuer")
	if err := svc.LockApplication(ctx, id); err != nil {
		t.Fatal(err)
	}
	err := svc.LockApplication(ctx, id)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("lost invalid transition identity: %v", err)
	}
}
