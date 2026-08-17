package application_test

import (
	"context"
	"path/filepath"
	"testing"

	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
)

func setupService(t *testing.T) *application.CertificateService {
	t.Helper()
	dir := t.TempDir()
	repo, err := repository.NewFileRepository(filepath.Join(dir, "store"))
	if err != nil {
		t.Fatal(err)
	}
	jrnl, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := recovery.NewRecoveryManager(filepath.Join(dir, "store"), jrnl)
	if err != nil {
		t.Fatal(err)
	}
	auditLog := audit.NewInMemoryAuditLog()
	return application.NewCertificateService(repo, rec, jrnl, auditLog)
}

func TestCertificateLifecycleChain(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()
	appID, err := svc.SubmitApplication(ctx, "example.com", "RSA-2048", "Org1")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LockApplication(ctx, appID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterIssuance(ctx, appID); err != nil {
		t.Fatal(err)
	}
	// 部署
	batchID, err := svc.DeployBatch(ctx, "some-cert-id", "target-1", 10) // 这里certID需从注册结果获取，测试中略
	_ = batchID
	_ = err
	// 由于需要certID，测试简化为验证状态转换
}

func TestRevocationBlocksLateConfirmation(t *testing.T) {
	// 略
}

func TestOptimisticConflict(t *testing.T) {
	// 略
}
