package domain

import "time"

type ActivationConfirmation struct {
	ID            string
	CertificateID string
	TargetID      string
	BatchID       string
	Digest        SubjectDigest
	ConfirmedAt   time.Time
	Version       int64
}
