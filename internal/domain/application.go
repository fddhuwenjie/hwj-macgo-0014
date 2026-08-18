package domain

import (
	"time"
)

type ApplicationStatus string

const (
	ApplicationDraft      ApplicationStatus = "DRAFT"
	ApplicationLocked     ApplicationStatus = "LOCKED"
	ApplicationIssued     ApplicationStatus = "ISSUED"
	ApplicationDeploying  ApplicationStatus = "DEPLOYING"
	ApplicationActive     ApplicationStatus = "ACTIVE"
	ApplicationRenewing   ApplicationStatus = "RENEWING"
	ApplicationRevoked    ApplicationStatus = "REVOKED"
)

type Application struct {
	ID               string
	Subject          string
	SubjectDigest    SubjectDigest
	IssuanceConfigID string
	Status           ApplicationStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

func (a *Application) CanTransitionTo(next ApplicationStatus) bool {
	switch a.Status {
	case ApplicationDraft:
		return next == ApplicationLocked || next == ApplicationRevoked
	case ApplicationLocked:
		return next == ApplicationIssued || next == ApplicationRevoked
	case ApplicationIssued:
		return next == ApplicationDeploying || next == ApplicationRevoked
	case ApplicationDeploying:
		return next == ApplicationActive || next == ApplicationRenewing || next == ApplicationRevoked
	case ApplicationActive:
		return next == ApplicationRenewing || next == ApplicationRevoked
	case ApplicationRenewing:
		return next == ApplicationActive || next == ApplicationRevoked
	case ApplicationRevoked:
		return false
	default:
		return false
	}
}
