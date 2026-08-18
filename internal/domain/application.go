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
		// 锁定的申请可以登记签发（LOCKED -> ISSUED），也可以被撤销。
		// 与 state_machine.go 中 NextStatusForEvent("issue", LOCKED) = ISSUED 保持一致。
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
