package application_test

import (
	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
	"context"
	"path/filepath"
	"testing"
)

func TestBug01LockedApplicationCanIssue(t *testing.T) {
	dir := t.TempDir()
	repo, _ := repository.NewFileRepository(filepath.Join(dir, "store"))
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(filepath.Join(dir, "store"), j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	id, err := svc.SubmitApplication(ctx, "example.test", "RSA-2048", "issuer")
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.LockApplication(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err = svc.RegisterIssuance(ctx, id); err != nil {
		t.Fatalf("issue locked application: %v", err)
	}
	app, err := repo.Applications.Get(ctx, id)
	if err != nil || app.Status != domain.ApplicationIssued {
		t.Fatalf("application state: %#v %v", app, err)
	}
	certs, err := repo.Certificates.ListByApplication(ctx, id)
	if err != nil || len(certs) != 1 {
		t.Fatalf("certificates: %d %v", len(certs), err)
	}
}
