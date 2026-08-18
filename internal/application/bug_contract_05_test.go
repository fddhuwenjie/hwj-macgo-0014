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

func TestBug05RenewalPlanCertificateLink(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	repo, _ := repository.NewFileRepository(store)
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(store, j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	app := &domain.Application{ID: "app", Status: domain.ApplicationActive, Version: 1}
	cert := &domain.CertificateVersion{ID: "cert", ApplicationID: app.ID, IssuanceConfigID: "cfg", Status: "ACTIVE", NotAfter: time.Now().AddDate(0, 2, 0), Version: 1}
	_ = repo.Applications.Create(ctx, app)
	_ = repo.Certificates.Create(ctx, cert)
	if _, err := svc.RenewCertificate(ctx, cert.ID); err != nil {
		t.Fatal(err)
	}
	plans, err := repo.Renewals.ListPlans(ctx)
	wantDue := cert.NotAfter.AddDate(0, 0, -30)
	if err != nil || len(plans) != 1 || plans[0].CertificateID != cert.ID || !plans[0].DueAt.Equal(wantDue) {
		t.Fatalf("renewal plans: %#v %v", plans, err)
	}
	gotApp, err := repo.Applications.Get(ctx, app.ID)
	if err != nil || gotApp.Status != domain.ApplicationRenewing || gotApp.Version != 2 {
		t.Fatalf("application after renewal: %#v %v", gotApp, err)
	}
}
