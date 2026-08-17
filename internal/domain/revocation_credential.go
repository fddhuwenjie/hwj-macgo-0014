package domain

import "time"

type RevocationCredential struct {
	ID            string
	CertificateID string
	ReasonCode    int
	RevokedAt     time.Time
	IssuedTo      string
	Version       int64
}
