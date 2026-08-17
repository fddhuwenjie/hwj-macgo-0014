package domain

import "time"

type ValidityObservation struct {
	ID            string
	CertificateID string
	NotBefore     time.Time
	NotAfter      time.Time
	OverlapsWith  []string
	GapDays       int
	ExpiringSoon  bool
	RecordedAt    time.Time
}
