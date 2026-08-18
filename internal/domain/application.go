package domain

import (
	"time"
)

type ApplicationStatus string

const (
	ApplicationDraft     ApplicationStatus = "DRAFT"
	ApplicationLocked    ApplicationStatus = "LOCKED"
	ApplicationIssued    ApplicationStatus = "ISSUED"
	ApplicationDeploying ApplicationStatus = "DEPLOYING"
	ApplicationActive    ApplicationStatus = "ACTIVE"
	ApplicationRenewing  ApplicationStatus = "RENEWING"
	ApplicationRevoked   ApplicationStatus = "REVOKED"
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

func (a *Application) ApplyLock(at time.Time) error {
	// Locking is a one-shot transition out of DRAFT. An application that is
	// already LOCKED is not re-locked, so a concurrent or repeated lock attempt
	// never advances the version a second time or produces a duplicate audit.
	// This matches CanTransitionTo and NextStatusForEvent("lock", ...), which
	// only allow DRAFT -> LOCKED.
	if a.Status != ApplicationDraft {
		return ErrInvalidTransition
	}
	a.Status = ApplicationLocked
	a.UpdatedAt = at
	a.Version++
	return nil
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
