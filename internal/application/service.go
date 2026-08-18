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
	confs, err := s.repo.Deployments.ListConfirmations(ctx, certID)
	if err != nil {
		return err
	}
	for _, c := range confs {
		if c.TargetID == targetID && c.Digest == digest {
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
	batch.ActivatedCount++
	batch.UpdatedAt = time.Now().UTC()
	batch.Version++
	if batch.ActivatedCount >= batch.TotalCount {
		batch.Status = "CONFIRMED"
		app, err := s.repo.Applications.Get(ctx, cert.ApplicationID)
		if err == nil && app.Status == domain.ApplicationDeploying {
			app.Status = domain.ApplicationActive
			app.UpdatedAt = time.Now().UTC()
			app.Version++
			if err := s.repo.Applications.Update(ctx, app); err != nil {
				return err
			}
		}
	} else {
		batch.Status = "PARTIAL"
	}
	if err := s.repo.Deployments.UpdateBatch(ctx, batch); err != nil {
		return err
	}
	s.auditLog.Record(ctx, "ConfirmActivation", conf.ID)
	return nil
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
