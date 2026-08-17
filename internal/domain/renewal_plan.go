package domain

import "time"

type RenewalPlan struct {
	ID            string
	CertificateID string
	DueAt         time.Time
	Status        string // "PENDING", "COMPLETED", "CANCELLED"
	Config        IssuanceConfig
	CreatedAt     time.Time
	Version       int64
}
