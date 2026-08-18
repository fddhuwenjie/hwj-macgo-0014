package application_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
)

// setupActivationService mirrors the bug-contract test's direct-repo wiring so
// individual confirmation scenarios can be constructed without driving the full
// application lifecycle.
func setupActivationService(t *testing.T) (*application.CertificateService, *repository.FileRepository) {
	t.Helper()
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
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	return svc, repo
}

// A second confirmation for the SAME (certificate, batch, target, digest) must
// be idempotent: it returns success without creating another record and without
// advancing the batch counter a second time.
func TestConfirmActivationIdempotentSameScope(t *testing.T) {
	svc, repo := setupActivationService(t)
	ctx := context.Background()
	digest := domain.NewSubjectDigest("subject")
	now := time.Now()

	app := &domain.Application{ID: "app-1", Status: domain.ApplicationDeploying, Version: 1}
	cert := &domain.CertificateVersion{ID: "cert-1", ApplicationID: app.ID, SubjectDigest: digest, Status: "ACTIVE", NotBefore: now, NotAfter: now.Add(time.Hour), Version: 1}
	// Total 2 so a single confirmation lands on PARTIAL and does not cascade the app.
	batch := &domain.DeploymentBatch{ID: "batch-1", CertificateID: cert.ID, TargetID: "target-1", Generation: 1, Status: "PENDING", TotalCount: 2, Version: 1}
	_ = repo.Applications.Create(ctx, app)
	_ = repo.Certificates.Create(ctx, cert)
	_ = repo.Deployments.CreateBatch(ctx, batch)

	if err := svc.ConfirmActivation(ctx, batch.ID, cert.ID, batch.TargetID, digest); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	// Duplicate: same cert, batch, target, digest.
	if err := svc.ConfirmActivation(ctx, batch.ID, cert.ID, batch.TargetID, digest); err != nil {
		t.Fatalf("duplicate confirm: %v", err)
	}

	got, err := repo.Deployments.GetBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActivatedCount != 1 {
		t.Fatalf("duplicate confirmation advanced the counter: count=%d", got.ActivatedCount)
	}
	if got.Status != "PARTIAL" {
		t.Fatalf("unexpected batch status: %s", got.Status)
	}
	confs, _ := repo.Deployments.ListConfirmations(ctx, cert.ID)
	if len(confs) != 1 {
		t.Fatalf("expected 1 confirmation, got %d", len(confs))
	}
}

// The same certificate + target + digest on a DIFFERENT batch must NOT be
// deduped: each batch keeps its own confirmation and counter, which is why
// BatchID is part of the idempotency key.
func TestConfirmActivationDifferentBatchNotDeduped(t *testing.T) {
	svc, repo := setupActivationService(t)
	ctx := context.Background()
	digest := domain.NewSubjectDigest("subject")
	now := time.Now()

	app := &domain.Application{ID: "app-1", Status: domain.ApplicationDeploying, Version: 1}
	cert := &domain.CertificateVersion{ID: "cert-1", ApplicationID: app.ID, SubjectDigest: digest, Status: "ACTIVE", NotBefore: now, NotAfter: now.Add(time.Hour), Version: 1}
	batch1 := &domain.DeploymentBatch{ID: "batch-1", CertificateID: cert.ID, TargetID: "target-1", Generation: 1, Status: "PENDING", TotalCount: 1, Version: 1}
	batch2 := &domain.DeploymentBatch{ID: "batch-2", CertificateID: cert.ID, TargetID: "target-1", Generation: 1, Status: "PENDING", TotalCount: 1, Version: 1}
	_ = repo.Applications.Create(ctx, app)
	_ = repo.Certificates.Create(ctx, cert)
	_ = repo.Deployments.CreateBatch(ctx, batch1)
	_ = repo.Deployments.CreateBatch(ctx, batch2)

	if err := svc.ConfirmActivation(ctx, batch1.ID, cert.ID, batch1.TargetID, digest); err != nil {
		t.Fatalf("confirm batch-1: %v", err)
	}
	// Same cert, target, digest but a different batch: must not be treated as a duplicate.
	if err := svc.ConfirmActivation(ctx, batch2.ID, cert.ID, batch2.TargetID, digest); err != nil {
		t.Fatalf("confirm batch-2: %v", err)
	}

	for _, id := range []string{batch1.ID, batch2.ID} {
		got, err := repo.Deployments.GetBatch(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.ActivatedCount != 1 || got.Status != "CONFIRMED" {
			t.Fatalf("batch %s not advanced independently: %#v", id, got)
		}
	}
	confs, _ := repo.Deployments.ListConfirmations(ctx, cert.ID)
	if len(confs) != 2 {
		t.Fatalf("expected 2 confirmations (one per batch), got %d", len(confs))
	}
}
