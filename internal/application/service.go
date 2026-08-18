package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"certarchive/internal/audit"
	"certarchive/internal/domain"
	"certarchive/internal/journal"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
)

type CertificateService struct {
	repo     *repository.FileRepository
	rec      *recovery.RecoveryManager
	jrnl     *journal.FileJournal
	auditLog *audit.InMemoryAuditLog
}

func NewCertificateService(repo *repository.FileRepository, rec *recovery.RecoveryManager, jrnl *journal.FileJournal, auditLog *audit.InMemoryAuditLog) *CertificateService {
	return &CertificateService{repo: repo, rec: rec, jrnl: jrnl, auditLog: auditLog}
}

func (s *CertificateService) SubmitApplication(ctx context.Context, subject, keyAlg, issuer string) (string, error) {
	if subject == "" || keyAlg == "" || issuer == "" {
		return "", domain.ErrInvalidArgument
	}
	if strings.TrimSpace(subject) == "" {
		return "", domain.ErrInvalidArgument
	}
	if !isValidKeyAlgorithm(keyAlg) {
		return "", domain.ErrInvalidArgument
	}
	app := &domain.Application{
		ID:               domain.NewID(),
		Subject:          subject,
		SubjectDigest:    domain.NewSubjectDigest(subject),
		IssuanceConfigID: "default-config",
		Status:           domain.ApplicationDraft,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		Version:          1,
	}
	if err := s.repo.Applications.Create(ctx, app); err != nil {
		return "", err
	}
	s.auditLog.Record(ctx, "SubmitApplication", app.ID)
	return app.ID, nil
}

func (s *CertificateService) LockApplication(ctx context.Context, id string) error {
	app, err := s.repo.Applications.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := domain.ValidateTransition(app.Status, domain.ApplicationLocked); err != nil {
		return err
	}
	oldStatus := app.Status
	app.Status = domain.ApplicationLocked
	app.UpdatedAt = time.Now().UTC()
	app.Version++
	if err := s.repo.Applications.Update(ctx, app); err != nil {
		return err
	}
	s.auditLog.Record(ctx, "LockApplication", fmt.Sprintf("%s:%s->%s", id, oldStatus, app.Status))
	return nil
}

func (s *CertificateService) RegisterIssuance(ctx context.Context, appID string) error {
	app, err := s.repo.Applications.Get(ctx, appID)
	if err != nil {
		return err
	}
	if err := domain.ValidateTransition(app.Status, domain.ApplicationIssued); err != nil {
		return err
	}
	notBefore := time.Now().UTC()
	notAfter := notBefore.AddDate(0, 0, 365)
	if notAfter.Before(notBefore) {
		return domain.ErrInvalidArgument
	}
	cert := &domain.CertificateVersion{
		ID:               domain.NewID(),
		ApplicationID:    appID,
		IssuanceConfigID: app.IssuanceConfigID,
		SubjectDigest:    app.SubjectDigest,
		SerialNumber:     domain.NewSerial(),
		NotBefore:        notBefore,
		NotAfter:         notAfter,
		Status:           "ACTIVE",
		CreatedAt:        time.Now().UTC(),
		Version:          1,
	}
	if err := s.repo.Certificates.Create(ctx, cert); err != nil {
		return err
	}
	app.Status = domain.ApplicationIssued
	app.UpdatedAt = time.Now().UTC()
	app.Version++
	if err := s.repo.Applications.Update(ctx, app); err != nil {
		return err
	}
	s.auditLog.Record(ctx, "RegisterIssuance", appID+"/"+cert.ID)
	return nil
}

func (s *CertificateService) DeployBatch(ctx context.Context, certID, targetID string, count int) (string, error) {
	if count <= 0 {
		return "", domain.ErrInvalidArgument
	}
	cert, err := s.repo.Certificates.Get(ctx, certID)
	if err != nil {
		return "", err
	}
	if cert.Status == "REVOKED" {
		return "", domain.ErrRevoked
	}
	target, err := s.repo.Targets.Get(ctx, targetID)
	if err != nil {
		return "", err
	}
	if target.Capacity <= 0 {
		return "", domain.ErrInvalidArgument
	}
	batch := &domain.DeploymentBatch{
		ID:            domain.NewID(),
		CertificateID: certID,
		TargetID:      targetID,
		Generation:    cert.Version,
		Status:        "PENDING",
		TotalCount:    count,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		Version:       1,
	}
	if err := domain.CheckDeploymentCapacity(batch, target); err != nil {
		return "", err
	}
	if err := s.repo.Deployments.CreateBatch(ctx, batch); err != nil {
		return "", err
	}
	app, err := s.repo.Applications.Get(ctx, cert.ApplicationID)
	if err == nil && app.Status == domain.ApplicationIssued {
		app.Status = domain.ApplicationDeploying
		app.UpdatedAt = time.Now().UTC()
		app.Version++
		if err := s.repo.Applications.Update(ctx, app); err != nil {
			return "", err
		}
	}
	s.auditLog.Record(ctx, "DeployBatch", batch.ID)
	return batch.ID, nil
}

func (s *CertificateService) ConfirmActivation(ctx context.Context, batchID, certID, targetID string, digest domain.SubjectDigest) error {
	// 幂等键作用域为（证书、批次、目标、摘要）：只有同一证书在同一批次、
	// 同一目标上以同一摘要确认过，才视为重复确认并直接返回成功。若仅按
	// 目标+摘要去重，会把另一证书（或同一证书的另一批次）的确认记录误判
	// 为本次确认，导致批次计数不增加却谎报幂等成功。
	confs, err := s.repo.Deployments.ListConfirmations(ctx, certID)
	if err != nil {
		return err
	}
	for _, c := range confs {
		if c.CertificateID == certID && c.BatchID == batchID &&
			c.TargetID == targetID && c.Digest == digest {
			return nil
		}
	}
	batch, err := s.repo.Deployments.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	cert, err := s.repo.Certificates.Get(ctx, certID)
	if err != nil {
		return err
	}
	if cert.Status == "REVOKED" {
		return domain.ErrRevoked
	}
	if batch.CertificateID != certID || batch.TargetID != targetID {
		return domain.ErrInvalidArgument
	}
	if batch.Generation != cert.Version {
		return domain.ErrStaleRevision
	}
	if digest != cert.SubjectDigest {
		return domain.ErrInvalidArgument
	}
	conf := &domain.ActivationConfirmation{
		ID:            domain.NewID(),
		CertificateID: certID,
		TargetID:      targetID,
		BatchID:       batchID,
		Digest:        digest,
		ConfirmedAt:   time.Now().UTC(),
		Version:       1,
	}
	if err := s.repo.Deployments.CreateConfirmation(ctx, conf); err != nil {
		return err
	}
	// 推进批次计数。若失败，回滚刚写入的确认记录，避免留下"有确认记录但
	// 计数未增加"的部分更新——否则重试时会因幂等命中而静默返回，批次被卡住。
	prevStatus := batch.Status
	batch.ActivatedCount++
	batch.UpdatedAt = time.Now().UTC()
	batch.Version++
	if batch.ActivatedCount >= batch.TotalCount {
		batch.Status = "CONFIRMED"
	} else {
		batch.Status = "PARTIAL"
	}
	if err := s.repo.Deployments.UpdateBatch(ctx, batch); err != nil {
		if rbErr := s.repo.Deployments.DeleteConfirmation(ctx, conf.ID); rbErr != nil {
			return fmt.Errorf("update batch failed: %w; rollback confirmation failed: %v", err, rbErr)
		}
		return err
	}
	// 批次确认完成后级联推进应用状态。若失败，回滚批次计数与确认记录，使
	// 相关状态、查询结果与持久化数据保持一致，调用方可整体重试。
	if batch.Status == "CONFIRMED" {
		if err := s.activateApplication(ctx, cert.ApplicationID); err != nil {
			if rbErr := s.rollbackBatchAndConfirmation(ctx, batch, prevStatus, conf.ID); rbErr != nil {
				return fmt.Errorf("activate application failed: %w; rollback failed: %v", err, rbErr)
			}
			return err
		}
	}
	s.auditLog.Record(ctx, "ConfirmActivation", conf.ID)
	return nil
}

// activateApplication 把处于 DEPLOYING 状态的应用推进为 ACTIVE。应用缺失或
// 不在 DEPLOYING 状态时不视为错误（与历史行为一致）；仅在成功读取到
// DEPLOYING 应用时才尝试更新，并返回更新结果。
func (s *CertificateService) activateApplication(ctx context.Context, appID string) error {
	app, err := s.repo.Applications.Get(ctx, appID)
	if err != nil {
		return nil
	}
	if app.Status != domain.ApplicationDeploying {
		return nil
	}
	app.Status = domain.ApplicationActive
	app.UpdatedAt = time.Now().UTC()
	app.Version++
	return s.repo.Applications.Update(ctx, app)
}

// rollbackBatchAndConfirmation 在级联推进应用状态失败时，将批次计数与状态恢复
// 到推进前的取值，并删除刚写入的确认记录，保证持久化状态不留部分更新。先恢复
// 批次再删除确认：即使回滚中途失败，残留的也是"确认存在但计数偏低"的可检测
// 状态，而非会引发重复计数的"计数偏高"状态。
func (s *CertificateService) rollbackBatchAndConfirmation(ctx context.Context, batch *domain.DeploymentBatch, prevStatus string, confID string) error {
	batch.ActivatedCount--
	batch.Status = prevStatus
	batch.Version++ // 磁盘上已是推进后的版本，再递增以通过乐观锁校验
	batch.UpdatedAt = time.Now().UTC()
	if err := s.repo.Deployments.UpdateBatch(ctx, batch); err != nil {
		return err
	}
	return s.repo.Deployments.DeleteConfirmation(ctx, confID)
}

func (s *CertificateService) RenewCertificate(ctx context.Context, certID string) (string, error) {
	cert, err := s.repo.Certificates.Get(ctx, certID)
	if err != nil {
		return "", err
	}
	if cert.Status == "REVOKED" {
		return "", domain.ErrRevoked
	}
	app, err := s.repo.Applications.Get(ctx, cert.ApplicationID)
	if err != nil {
		return "", err
	}
	if app.Status != domain.ApplicationActive {
		return "", domain.ErrInvalidTransition
	}
	plan := &domain.RenewalPlan{
		ID:            domain.NewID(),
		CertificateID: certID,
		DueAt:         cert.NotAfter.AddDate(0, 0, -30),
		Status:        "PENDING",
		Config:        domain.IssuanceConfig{ID: cert.IssuanceConfigID},
		CreatedAt:     time.Now().UTC(),
		Version:       1,
	}
	if err := s.repo.Renewals.CreatePlan(ctx, plan); err != nil {
		return "", err
	}
	app.Status = domain.ApplicationRenewing
	app.UpdatedAt = time.Now().UTC()
	app.Version++
	if err := s.repo.Applications.Update(ctx, app); err != nil {
		return "", err
	}
	s.auditLog.Record(ctx, "RenewCertificate", plan.ID)
	return plan.ID, nil
}

func (s *CertificateService) RevokeCertificate(ctx context.Context, certID string, reason int) error {
	cert, err := s.repo.Certificates.Get(ctx, certID)
	if err != nil {
		return err
	}
	if cert.Status == "REVOKED" {
		return domain.ErrRevoked
	}
	cred := &domain.RevocationCredential{
		ID:            domain.NewID(),
		CertificateID: certID,
		ReasonCode:    reason,
		RevokedAt:     time.Now().UTC(),
		IssuedTo:      cert.SubjectDigest.String(),
		Version:       1,
	}
	if err := s.repo.Revocations.CreateCredential(ctx, cred); err != nil {
		return err
	}
	cert.Status = "REVOKED"
	cert.Version++
	if err := s.repo.Certificates.Update(ctx, cert); err != nil {
		return err
	}
	app, err := s.repo.Applications.Get(ctx, cert.ApplicationID)
	if err == nil {
		app.Status = domain.ApplicationRevoked
		app.UpdatedAt = time.Now().UTC()
		app.Version++
		if err := s.repo.Applications.Update(ctx, app); err != nil {
			return err
		}
	}
	s.auditLog.Record(ctx, "RevokeCertificate", certID+"/"+cred.ID)
	return nil
}

func isValidKeyAlgorithm(alg string) bool {
	switch alg {
	case "RSA-2048", "RSA-4096", "ECDSA-P256", "ECDSA-P384":
		return true
	default:
		return false
	}
}
