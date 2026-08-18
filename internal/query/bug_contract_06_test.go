package query_test

import (
	"certarchive/internal/domain"
	"certarchive/internal/query"
	"certarchive/internal/repository"
	"context"
	"testing"
)

func TestBug06PendingDeploymentIsGap(t *testing.T) {
	repo, _ := repository.NewFileRepository(t.TempDir())
	ctx := context.Background()
	batches := []*domain.DeploymentBatch{
		{ID: "gap-partial", CertificateID: "cert", TargetID: "target-1", Status: "PARTIAL", ActivatedCount: 1, TotalCount: 3, Version: 1},
		{ID: "gap-pending", CertificateID: "cert", TargetID: "target-1", Status: "PENDING", ActivatedCount: 0, TotalCount: 2, Version: 1},
		{ID: "failed", CertificateID: "cert", TargetID: "target-1", Status: "FAILED", ActivatedCount: 0, TotalCount: 2, Version: 1},
		{ID: "complete", CertificateID: "cert", TargetID: "target-1", Status: "CONFIRMED", ActivatedCount: 2, TotalCount: 2, Version: 1},
	}
	for _, batch := range batches {
		if err := repo.Deployments.CreateBatch(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	got, err := query.NewQueryService(repo).FindDeploymentGaps(ctx)
	if err != nil || len(got) != 2 || got[0] != "gap-partial" || got[1] != "gap-pending" {
		t.Fatalf("deployment gaps: %#v %v", got, err)
	}
}
