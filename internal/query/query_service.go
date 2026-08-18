package query

import (
	"context"
	"sort"
	"time"

	"certarchive/internal/domain"
	"certarchive/internal/repository"
)

type QueryService struct {
	repo *repository.FileRepository
}

func NewQueryService(repo *repository.FileRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (q *QueryService) ListExpiringSoon(ctx context.Context, days int) ([]*domain.CertificateVersion, error) {
	certs, err := q.repo.Certificates.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	threshold := now.AddDate(0, 0, days)
	var result []*domain.CertificateVersion
	for _, c := range certs {
		if c.Status == "ACTIVE" && c.NotAfter.Before(threshold) && c.NotAfter.After(now) {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NotAfter.Before(result[j].NotAfter)
	})
	return result, nil
}

func (q *QueryService) FindValidityConflicts(ctx context.Context) ([]domain.ValidityObservation, error) {
	certs, err := q.repo.Certificates.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	var observations []domain.ValidityObservation
	for i := 0; i < len(certs); i++ {
		for j := i + 1; j < len(certs); j++ {
			a, b := certs[i], certs[j]
			if a.ApplicationID == b.ApplicationID {
				if domain.CheckValidityOverlap(a.NotBefore, a.NotAfter, b.NotBefore, b.NotAfter) {
					observations = append(observations, domain.ValidityObservation{
						ID:            domain.NewID(),
						CertificateID: a.ID,
						NotBefore:     a.NotBefore,
						NotAfter:      a.NotAfter,
						OverlapsWith:  []string{b.ID},
						RecordedAt:    time.Now(),
					})
				}
			}
		}
	}
	return observations, nil
}

func (q *QueryService) FindDeploymentGaps(ctx context.Context) ([]string, error) {
	batches, err := q.repo.Deployments.ListBatches(ctx)
	if err != nil {
		return nil, err
	}
	// 部署缺口口径：未完成（已激活计数小于总数）且非已确认、非失败的批次。
	// 这覆盖待处理（PENDING）与部分激活（PARTIAL）两类未完成批次，
	// 排除已确认（CONFIRMED，已激活完毕且进入终态）与失败（FAILED）批次。
	// 状态过滤放在查询层而非仓储层，使历史数据中各状态的批次均可重建。
	var gaps []string
	for _, b := range batches {
		if b.IsGap() {
			gaps = append(gaps, b.ID)
		}
	}
	return gaps, nil
}

func (q *QueryService) ListRevocationImpact(ctx context.Context, certID string) ([]string, error) {
	confs, err := q.repo.Deployments.ListConfirmations(ctx, certID)
	if err != nil {
		return nil, err
	}
	var impacted []string
	for _, c := range confs {
		impacted = append(impacted, c.TargetID)
	}
	return impacted, nil
}
