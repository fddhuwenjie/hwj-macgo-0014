package domain

import "time"

type CertificateVersion struct {
	ID                string
	ApplicationID     string
	IssuanceConfigID  string
	SubjectDigest     SubjectDigest
	SerialNumber      string
	NotBefore         time.Time
	NotAfter          time.Time
	Status            string // "ACTIVE", "REVOKED", "EXPIRED", "RENEWED"
	CreatedAt         time.Time
	Version           int64
}
