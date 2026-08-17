package domain

import "time"

type DeploymentBatch struct {
	ID            string
	CertificateID string
	TargetID      string
	Generation    int64
	Status        string // "PENDING", "PARTIAL", "CONFIRMED", "FAILED"
	ActivatedCount int
	TotalCount    int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}
