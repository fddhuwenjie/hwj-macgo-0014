package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrOptimisticConflict = errors.New("optimistic conflict")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrRevoked            = errors.New("certificate revoked")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrStaleRevision      = errors.New("stale revision")
	ErrDeploymentFull     = errors.New("deployment batch full")
	ErrOverlapDetected    = errors.New("validity overlap detected")
	ErrDependencyMissing  = errors.New("dependency missing")
)
