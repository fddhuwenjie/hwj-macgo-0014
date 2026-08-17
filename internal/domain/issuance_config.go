package domain

import "time"

type IssuanceConfig struct {
	ID             string
	KeyAlgorithm   string
	KeySize        int
	ValidityDays   int
	AllowedUsages  []string
	IssuerName     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}
