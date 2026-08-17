package domain

import (
	"fmt"
)

func ValidateTransition(from, to ApplicationStatus) error {
	app := Application{Status: from}
	if !app.CanTransitionTo(to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}
	return nil
}

func NextStatusForEvent(event string, current ApplicationStatus) (ApplicationStatus, error) {
	mapping := map[string]map[ApplicationStatus]ApplicationStatus{
		"lock": {
			ApplicationDraft: ApplicationLocked,
		},
		"issue": {
			ApplicationLocked: ApplicationIssued,
		},
		"deploy": {
			ApplicationIssued: ApplicationDeploying,
		},
		"activate": {
			ApplicationDeploying: ApplicationActive,
		},
		"renew": {
			ApplicationActive: ApplicationRenewing,
		},
		"activate_renewal": {
			ApplicationRenewing: ApplicationActive,
		},
		"revoke": {
			ApplicationDraft:      ApplicationRevoked,
			ApplicationLocked:     ApplicationRevoked,
			ApplicationIssued:     ApplicationRevoked,
			ApplicationDeploying:  ApplicationRevoked,
			ApplicationActive:     ApplicationRevoked,
			ApplicationRenewing:   ApplicationRevoked,
		},
	}
	if transitions, ok := mapping[event]; ok {
		if next, exists := transitions[current]; exists {
			return next, nil
		}
	}
	return "", fmt.Errorf("%w for event %s from %s", ErrInvalidTransition, event, current)
}
