package domain

import (
	"time"
)

func CheckValidityOverlap(aNotBefore, aNotAfter, bNotBefore, bNotAfter time.Time) bool {
	return aNotBefore.Before(bNotAfter) && aNotAfter.After(bNotBefore)
}

func CheckDeploymentCapacity(batch *DeploymentBatch, target *DeploymentTarget) error {
	if batch.TotalCount >= target.Capacity {
		return ErrDeploymentFull
	}
	return nil
}

func CheckRenewalDeadline(plan *RenewalPlan, now time.Time) error {
	if now.After(plan.DueAt) {
		return ErrInvalidArgument
	}
	return nil
}

func IsRevoked(status string) bool {
	return status == "REVOKED"
}
