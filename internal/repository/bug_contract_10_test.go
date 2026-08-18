package repository_test

import (
	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBug10RevocationCredentialScopeAndReason(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	repo, _ := repository.NewFileRepository(store)
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(store, j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	app := &domain.Application{ID: "app", Status: domain.ApplicationActive, Version: 1}
	cert := &domain.CertificateVersion{ID: "cert", ApplicationID: app.ID, SubjectDigest: domain.NewSubjectDigest("subject"), Status: "ACTIVE", NotAfter: time.Now().Add(time.Hour), Version: 1}
	other := &domain.CertificateVersion{ID: "other", ApplicationID: app.ID, SubjectDigest: domain.NewSubjectDigest("other"), Status: "ACTIVE", NotAfter: time.Now().Add(time.Hour), Version: 1}
	_ = repo.Applications.Create(ctx, app)
	_ = repo.Certificates.Create(ctx, cert)
	_ = repo.Certificates.Create(ctx, other)
	if err := svc.RevokeCertificate(ctx, cert.ID, 7); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(store, "revocations"))
	if err != nil || len(files) != 1 {
		t.Fatalf("revocation files: %d %v", len(files), err)
	}
	raw, _ := os.ReadFile(filepath.Join(store, "revocations", files[0].Name()))
	var cred domain.RevocationCredential
	if err = json.Unmarshal(raw, &cred); err != nil {
		t.Fatal(err)
	}
	if cred.ReasonCode != 7 || cred.CertificateID != cert.ID {
		t.Fatalf("stored credential: %#v", cred)
	}
	got, err := repo.Revocations.ListByCertificate(ctx, cert.ID)
	if err != nil || len(got) != 1 || got[0].CertificateID != cert.ID {
		t.Fatalf("credential query: %#v %v", got, err)
	}
	if err := svc.RevokeCertificate(ctx, other.ID, 9); err != nil {
		t.Fatal(err)
	}
	got, err = repo.Revocations.ListByCertificate(ctx, cert.ID)
	if err != nil || len(got) != 1 || got[0].ReasonCode != 7 {
		t.Fatalf("certificate-scoped credentials: %#v %v", got, err)
	}
}
