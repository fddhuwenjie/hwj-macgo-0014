package query_test

import (
	"certarchive/internal/domain"
	"certarchive/internal/query"
	"certarchive/internal/repository"
	"context"
	"testing"
	"time"
)

func TestBug04ExpiringUniqueAndOrdered(t *testing.T) {
	repo, _ := repository.NewFileRepository(t.TempDir())
	ctx := context.Background()
	now := time.Now()
	expired := &domain.CertificateVersion{ID: "expired", Status: "ACTIVE", NotBefore: now.Add(-2 * time.Hour), NotAfter: now.Add(-time.Minute), Version: 1}
	early := &domain.CertificateVersion{ID: "early", Status: "ACTIVE", NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), Version: 1}
	late := &domain.CertificateVersion{ID: "late", Status: "ACTIVE", NotBefore: now.Add(-time.Hour), NotAfter: now.Add(2 * time.Hour), Version: 1}
	_ = repo.Certificates.Create(ctx, expired)
	_ = repo.Certificates.Create(ctx, late)
	_ = repo.Certificates.Create(ctx, early)
	got, err := query.NewQueryService(repo).ListExpiringSoon(ctx, 7)
	if err != nil || len(got) != 2 || got[0].ID != "early" || got[1].ID != "late" {
		t.Fatalf("expiring query: %#v %v", got, err)
	}
}
