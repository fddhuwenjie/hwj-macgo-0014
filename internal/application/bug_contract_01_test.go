package application_test

import (
	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
	"context"
	"os"
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

// 草稿申请不得直接登记签发（DRAFT -> ISSUED 非法），
// 且不得因此创建任何证书。
func TestBug01DraftApplicationCannotIssue(t *testing.T) {
	dir := t.TempDir()
	repo, _ := repository.NewFileRepository(filepath.Join(dir, "store"))
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(filepath.Join(dir, "store"), j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	id, err := svc.SubmitApplication(ctx, "draft.test", "RSA-2048", "issuer")
	if err != nil {
		t.Fatal(err)
	}
	// 未锁定，直接登记签发必须被拒绝。
	if err := svc.RegisterIssuance(ctx, id); err == nil {
		t.Fatal("expected RegisterIssuance on a DRAFT application to be rejected")
	}
	// 状态、查询结果与持久化数据保持一致：申请仍为 DRAFT，无证书。
	app, err := repo.Applications.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != domain.ApplicationDraft {
		t.Fatalf("application status = %s, want DRAFT", app.Status)
	}
	certs, err := repo.Certificates.ListByApplication(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 0 {
		t.Fatalf("expected 0 certificates for rejected draft, got %d", len(certs))
	}
}

// 已撤销申请不得再次登记签发（REVOKED -> ISSUED 非法），
// 且不得因此创建新证书。
func TestBug01RevokedApplicationCannotIssue(t *testing.T) {
	dir := t.TempDir()
	repo, _ := repository.NewFileRepository(filepath.Join(dir, "store"))
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(filepath.Join(dir, "store"), j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()
	id, err := svc.SubmitApplication(ctx, "revoked.test", "RSA-2048", "issuer")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LockApplication(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterIssuance(ctx, id); err != nil {
		t.Fatal(err)
	}
	certs, _ := repo.Certificates.ListByApplication(ctx, id)
	if len(certs) != 1 {
		t.Fatalf("setup: expected 1 certificate, got %d", len(certs))
	}
	// 撤销该证书，申请随之进入 REVOKED。
	if err := svc.RevokeCertificate(ctx, certs[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	// 撤销后再次登记签发必须被拒绝。
	if err := svc.RegisterIssuance(ctx, id); err == nil {
		t.Fatal("expected RegisterIssuance on a REVOKED application to be rejected")
	}
	// 不应产生新证书，仍只有撤销前的那一张。
	certsAfter, err := repo.Certificates.ListByApplication(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(certsAfter) != 1 {
		t.Fatalf("expected 1 certificate after rejected re-issue, got %d", len(certsAfter))
	}
	app, err := repo.Applications.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != domain.ApplicationRevoked {
		t.Fatalf("application status = %s, want REVOKED", app.Status)
	}
}

// 若登记签发过程中申请状态未能持久化（例如并发导致的乐观冲突，
// 或底层写入失败），已创建的证书必须被回滚，避免留下部分更新：
// 申请保持 LOCKED、无证书，重试可干净地重新签发。
func TestBug01RegisterIssuanceRollsBackOnUpdateFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directory test is ineffective when running as root")
	}
	dir := t.TempDir()
	repo, _ := repository.NewFileRepository(filepath.Join(dir, "store"))
	j, _ := journal.NewFileJournal(filepath.Join(dir, "journal"))
	rec, _ := recovery.NewRecoveryManager(filepath.Join(dir, "store"), j)
	svc := application.NewCertificateService(repo, rec, j, audit.NewInMemoryAuditLog())
	ctx := context.Background()

	id, err := svc.SubmitApplication(ctx, "rollback.test", "RSA-2048", "issuer")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LockApplication(ctx, id); err != nil {
		t.Fatal(err)
	}

	// 将 applications 目录置为只读：证书目录仍可写，故证书会被创建；
	// 随后申请状态更新因无法写入而失败，触发回滚。
	appDir := filepath.Join(dir, "store", "applications")
	if err := os.Chmod(appDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(appDir, 0755) // 恢复权限，便于 TempDir 清理

	if err := svc.RegisterIssuance(ctx, id); err == nil {
		t.Fatal("expected RegisterIssuance to fail when the application update cannot persist")
	}

	// 状态：申请仍为 LOCKED（目录只读但仍可读）。
	app, err := repo.Applications.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != domain.ApplicationLocked {
		t.Fatalf("application status = %s, want LOCKED (no partial update)", app.Status)
	}
	// 查询结果与持久化数据：证书已被回滚，应为空。
	certs, err := repo.Certificates.ListByApplication(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 0 {
		t.Fatalf("expected 0 certificates after rollback, got %d", len(certs))
	}
}

