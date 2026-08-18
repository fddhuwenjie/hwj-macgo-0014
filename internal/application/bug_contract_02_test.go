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
	"time"
)

func TestBug02ActivationIdempotencyCertificateScope(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	repo, _ := repository.NewFileRepository(store)
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(store, j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	digest := domain.NewSubjectDigest("subject")
	now := time.Now()
	app := &domain.Application{ID: "app-1", Status: domain.ApplicationDeploying, Version: 1}
	cert := &domain.CertificateVersion{ID: "cert-1", ApplicationID: app.ID, SubjectDigest: digest, Status: "ACTIVE", NotBefore: now, NotAfter: now.Add(time.Hour), Version: 1}
	_ = repo.Applications.Create(ctx, app)
	_ = repo.Certificates.Create(ctx, cert)
	batch := &domain.DeploymentBatch{ID: "batch-shared", CertificateID: cert.ID, TargetID: "target-1", Generation: 1, Status: "PENDING", TotalCount: 1, Version: 1}
	_ = repo.Deployments.CreateBatch(ctx, batch)
	_ = repo.Deployments.CreateConfirmation(ctx, &domain.ActivationConfirmation{ID: "foreign", CertificateID: "cert-2", BatchID: batch.ID, TargetID: batch.TargetID, Digest: digest, Version: 1})
	if err := svc.ConfirmActivation(ctx, batch.ID, cert.ID, batch.TargetID, digest); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Deployments.GetBatch(ctx, batch.ID)
	if err != nil || got.ActivatedCount != 1 || got.Status != "CONFIRMED" {
		t.Fatalf("batch not advanced: %#v %v", got, err)
	}
	confs, _ := repo.Deployments.ListConfirmations(ctx, cert.ID)
	if len(confs) != 1 || confs[0].CertificateID != cert.ID {
		t.Fatalf("certificate confirmation scope: %#v", confs)
	}
}
